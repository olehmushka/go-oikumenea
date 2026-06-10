// Package application holds the rank module's application service — the orchestrator the transport
// layer calls to read/mutate the single system-wide rank scheme, recording an audit row in the same
// transaction as each write (D-Audit). It depends on the domain port, the platform DB surface, and
// the audit service; it never imports the adapters package directly (the repository factory is
// injected by module.go). Rank is a directory attribute and never an authorization input.
package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// auditSubsystem labels the interim system actor for rank's admin writes. Until authorization (M7) +
// identity-federation (M8) resolve the acting person, these writes are recorded as a `system` action
// under this subsystem (the no-unaudited-mutation ground rule still holds). M7/M8 replace this with
// the resolved person actor.
const auditSubsystem = "rank-admin"

// Audit target types (the audited entity kinds; mirror the i18n entity_type slots).
const (
	targetSystem   = "rank_system"
	targetCategory = "rank_category"
	targetType     = "rank_type"
	targetRank     = "rank"
)

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an audited write (D-Audit). Injected by module.go so the application layer
// never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the rank application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
}

// NewService wires the service with the pool, the repository factory, and the audit service every
// write records into.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// Scheme is the whole rank scheme as flat, ordered lists; the transport assembles the nested
// system -> category -> type -> rank tree (and the localized names) for the response.
type Scheme struct {
	Systems    []domain.System
	Categories []domain.Category
	Types      []domain.Type
	Ranks      []domain.Rank
}

// GetScheme reads the whole scheme in seniority order (each list ordered by sort_order, code).
func (s *Service) GetScheme(ctx context.Context) (Scheme, error) {
	repo := s.newRepo(s.pool)
	systems, err := repo.ListSystems(ctx)
	if err != nil {
		return Scheme{}, err
	}
	categories, err := repo.ListCategories(ctx)
	if err != nil {
		return Scheme{}, err
	}
	types, err := repo.ListTypes(ctx)
	if err != nil {
		return Scheme{}, err
	}
	ranks, err := repo.ListRanks(ctx)
	if err != nil {
		return Scheme{}, err
	}
	return Scheme{Systems: systems, Categories: categories, Types: types, Ranks: ranks}, nil
}

// GetGrades reads the seeded standardized-grade comparator catalog (NATO STANAG 2116), already ordered
// by the comparability scale (tier then ordinal).
func (s *Service) GetGrades(ctx context.Context) ([]domain.Grade, error) {
	return s.newRepo(s.pool).ListGrades(ctx)
}

// ---------------------------------------------------------------- systems

// AddSystem validates and creates a rank system (the top level), then records the action. sortOrder nil
// appends last. country nil = a supranational system.
func (s *Service) AddSystem(ctx context.Context, code, name string, sortOrder *int, country *string) (domain.System, error) {
	sys := domain.System{Code: code, Name: name}
	if err := sys.Validate(); err != nil {
		return domain.System{}, err
	}
	var out domain.System
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertSystem(ctx, code, name, sortOrder, country)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "rank.system.create", targetSystem, created.ID, created)
	})
	return out, err
}

// UpdateSystem applies a partial change (rename/reorder/country) and records the action.
func (s *Service) UpdateSystem(ctx context.Context, id string, patch domain.SystemPatch) (domain.System, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.System{}, domain.ErrInvalid
	}
	var out domain.System
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateSystem(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "rank.system.update", targetSystem, id, updated)
	})
	return out, err
}

// ---------------------------------------------------------------- preset import

// Preset is a curated rank-system subtree (D-RankSystems): one system with its categories -> types
// (a tree) -> ranks, each carrying a stable `code` so an import is a code-keyed idempotent upsert. The
// transport maps the Conjure request to this shape.
type (
	Preset struct {
		System PresetSystem
	}
	PresetSystem struct {
		Code, Name string
		Country    *string
		SortOrder  *int
		Categories []PresetCategory
	}
	PresetCategory struct {
		Code, Name string
		SortOrder  *int
		Types      []PresetType
	}
	PresetType struct {
		Code, Name string
		SortOrder  *int
		Children   []PresetType
		Ranks      []PresetRank
	}
	PresetRank struct {
		Code, Name   string
		Abbreviation *string
		GradeCode    *string
		SortOrder    *int
	}
)

// ImportSummary reports how many scheme nodes an import created, updated, or skipped (already current).
type ImportSummary struct {
	Created int
	Updated int
	Skipped int
}

// ImportPreset applies a preset as a code-keyed, idempotent upsert in ONE transaction (D-RankSystems):
// existing nodes are matched by `code` within their parent and updated only where a field differs
// (otherwise skipped), new nodes are inserted, and nothing is ever deleted. The leaf-only rule holds —
// a type may carry children or ranks, not both. The whole import is one audited action. Re-importing an
// unchanged preset reports all-skipped (idempotent).
func (s *Service) ImportPreset(ctx context.Context, p Preset) (ImportSummary, error) {
	var sum ImportSummary
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		systemID, err := s.upsertSystem(ctx, repo, p.System, &sum)
		if err != nil {
			return err
		}
		for _, pc := range p.System.Categories {
			categoryID, err := s.upsertCategory(ctx, repo, systemID, pc, &sum)
			if err != nil {
				return err
			}
			for _, pt := range pc.Types {
				if err := s.upsertType(ctx, repo, categoryID, nil, pt, &sum); err != nil {
					return err
				}
			}
		}
		return s.record(ctx, tx, "rank.scheme.import", targetSystem, systemID,
			map[string]any{"system": p.System.Code, "summary": sum})
	})
	if err != nil {
		return ImportSummary{}, err
	}
	return sum, nil
}

func (s *Service) upsertSystem(ctx context.Context, repo domain.Repository, ps PresetSystem, sum *ImportSummary) (string, error) {
	if !validCodeName(ps.Code, ps.Name) {
		return "", domain.ErrInvalid
	}
	existing, err := repo.GetSystemByCode(ctx, ps.Code)
	if errors.Is(err, domain.ErrSystemNotFound) {
		created, err := repo.InsertSystem(ctx, ps.Code, ps.Name, ps.SortOrder, ps.Country)
		if err != nil {
			return "", err
		}
		sum.Created++
		return created.ID, nil
	}
	if err != nil {
		return "", err
	}
	patch := domain.SystemPatch{}
	changed := false
	if existing.Name != ps.Name {
		patch.Name = &ps.Name
		changed = true
	}
	if ps.SortOrder != nil && existing.SortOrder != *ps.SortOrder {
		patch.SortOrder = ps.SortOrder
		changed = true
	}
	if ps.Country != nil && existing.Country != *ps.Country {
		patch.Country = ps.Country
		changed = true
	}
	if !changed {
		sum.Skipped++
		return existing.ID, nil
	}
	if _, err := repo.UpdateSystem(ctx, existing.ID, patch); err != nil {
		return "", err
	}
	sum.Updated++
	return existing.ID, nil
}

func (s *Service) upsertCategory(ctx context.Context, repo domain.Repository, systemID string, pc PresetCategory, sum *ImportSummary) (string, error) {
	if !validCodeName(pc.Code, pc.Name) {
		return "", domain.ErrInvalid
	}
	existing, err := repo.GetCategoryByCode(ctx, systemID, pc.Code)
	if errors.Is(err, domain.ErrCategoryNotFound) {
		created, err := repo.InsertCategory(ctx, systemID, pc.Code, pc.Name, pc.SortOrder)
		if err != nil {
			return "", err
		}
		sum.Created++
		return created.ID, nil
	}
	if err != nil {
		return "", err
	}
	patch := domain.CategoryPatch{}
	changed := false
	if existing.Name != pc.Name {
		patch.Name = &pc.Name
		changed = true
	}
	if pc.SortOrder != nil && existing.SortOrder != *pc.SortOrder {
		patch.SortOrder = pc.SortOrder
		changed = true
	}
	if !changed {
		sum.Skipped++
		return existing.ID, nil
	}
	if _, err := repo.UpdateCategory(ctx, existing.ID, patch); err != nil {
		return "", err
	}
	sum.Updated++
	return existing.ID, nil
}

// upsertType upserts one type (matched by code among its siblings) and recurses into its children;
// ranks attach to leaf types only, so a type carrying children must not also carry ranks.
func (s *Service) upsertType(ctx context.Context, repo domain.Repository, categoryID string, parentTypeID *string, pt PresetType, sum *ImportSummary) error {
	if !validCodeName(pt.Code, pt.Name) {
		return domain.ErrInvalid
	}
	if len(pt.Children) > 0 && len(pt.Ranks) > 0 {
		return errors.Join(domain.ErrInvalid, errors.New("preset type has both children and ranks; ranks live on leaf types only"))
	}
	existing, err := repo.GetTypeByCode(ctx, categoryID, parentTypeID, pt.Code)
	var typeID string
	switch {
	case errors.Is(err, domain.ErrTypeNotFound):
		created, err := repo.InsertType(ctx, categoryID, parentTypeID, pt.Code, pt.Name, pt.SortOrder)
		if err != nil {
			return err
		}
		typeID = created.ID
		sum.Created++
	case err != nil:
		return err
	default:
		typeID = existing.ID
		patch := domain.TypePatch{}
		changed := false
		if existing.Name != pt.Name {
			patch.Name = &pt.Name
			changed = true
		}
		if pt.SortOrder != nil && existing.SortOrder != *pt.SortOrder {
			patch.SortOrder = pt.SortOrder
			changed = true
		}
		if changed {
			if _, err := repo.UpdateType(ctx, existing.ID, patch); err != nil {
				return err
			}
			sum.Updated++
		} else {
			sum.Skipped++
		}
	}
	for _, child := range pt.Children {
		if err := s.upsertType(ctx, repo, categoryID, &typeID, child, sum); err != nil {
			return err
		}
	}
	for _, pr := range pt.Ranks {
		if err := s.upsertRank(ctx, repo, typeID, pr, sum); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) upsertRank(ctx context.Context, repo domain.Repository, typeID string, pr PresetRank, sum *ImportSummary) error {
	if !validCodeName(pr.Code, pr.Name) {
		return domain.ErrInvalid
	}
	if err := s.validateGrade(ctx, repo, pr.GradeCode); err != nil {
		return err
	}
	existing, err := repo.GetRankByCode(ctx, typeID, pr.Code)
	if errors.Is(err, domain.ErrRankNotFound) {
		if _, err := repo.InsertRank(ctx, typeID, pr.Code, pr.Name, pr.Abbreviation, pr.GradeCode, pr.SortOrder); err != nil {
			return err
		}
		sum.Created++
		return nil
	}
	if err != nil {
		return err
	}
	patch := domain.RankPatch{}
	changed := false
	if existing.Name != pr.Name {
		patch.Name = &pr.Name
		changed = true
	}
	if pr.Abbreviation != nil && existing.Abbreviation != *pr.Abbreviation {
		patch.Abbreviation = pr.Abbreviation
		changed = true
	}
	if pr.GradeCode != nil && existing.GradeCode != *pr.GradeCode {
		patch.GradeCode = pr.GradeCode
		changed = true
	}
	if pr.SortOrder != nil && existing.SortOrder != *pr.SortOrder {
		patch.SortOrder = pr.SortOrder
		changed = true
	}
	if !changed {
		sum.Skipped++
		return nil
	}
	if _, err := repo.UpdateRank(ctx, existing.ID, patch); err != nil {
		return err
	}
	sum.Updated++
	return nil
}

// validCodeName guards a preset node's code/name shape (mirrors the domain node validators) before any
// DB work, so a malformed preset fails fast inside the import transaction.
func validCodeName(code, name string) bool {
	return (domain.Category{Code: code, Name: name}).Validate() == nil
}

// ---------------------------------------------------------------- categories

// AddCategory validates and creates a category under a system, then records the action. The owning
// system must exist and be active (else ErrSystemNotFound). sortOrder nil appends last.
func (s *Service) AddCategory(ctx context.Context, systemID, code, name string, sortOrder *int) (domain.Category, error) {
	c := domain.Category{Code: code, Name: name, SystemID: systemID}
	if err := c.Validate(); err != nil {
		return domain.Category{}, err
	}
	var out domain.Category
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetSystem(ctx, systemID); err != nil {
			return err // ErrSystemNotFound when the owning system is absent or soft-deleted
		}
		created, err := repo.InsertCategory(ctx, systemID, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "rank.category.create", targetCategory, created.ID, created)
	})
	return out, err
}

// UpdateCategory applies a partial change (rename/reorder) and records the action.
func (s *Service) UpdateCategory(ctx context.Context, id string, patch domain.CategoryPatch) (domain.Category, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.Category{}, domain.ErrInvalid
	}
	var out domain.Category
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateCategory(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "rank.category.update", targetCategory, id, updated)
	})
	return out, err
}

// ---------------------------------------------------------------- types

// AddType validates the request and creates a type, then records the action — all in one transaction.
// A type is rooted either directly under a category (parentTypeID nil) or under a parent type (it then
// inherits the parent's category, denormalized). Ranks live on leaf types only, so a parent type that
// already holds active ranks cannot gain child types.
func (s *Service) AddType(ctx context.Context, categoryID string, parentTypeID *string, code, name string, sortOrder *int) (domain.Type, error) {
	t := domain.Type{Code: code, Name: name, CategoryID: categoryID}
	if parentTypeID != nil {
		t.ParentTypeID = *parentTypeID
	}
	if err := t.Validate(); err != nil {
		return domain.Type{}, err
	}
	var out domain.Type
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		effectiveCategory := categoryID
		if parentTypeID != nil {
			parent, err := repo.GetType(ctx, *parentTypeID)
			if err != nil {
				return err // ErrTypeNotFound when the parent type is absent or soft-deleted
			}
			effectiveCategory = parent.CategoryID // nested types inherit (denormalize) the root category
			rankCount, err := repo.CountActiveRanks(ctx, *parentTypeID)
			if err != nil {
				return err
			}
			if rankCount > 0 {
				return errors.Join(domain.ErrInvalid, errors.New("parent type already holds ranks; ranks live on leaf types only"))
			}
		} else if _, err := repo.GetCategory(ctx, categoryID); err != nil {
			return err // ErrCategoryNotFound when the parent category is absent or soft-deleted
		}
		created, err := repo.InsertType(ctx, effectiveCategory, parentTypeID, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "rank.type.create", targetType, created.ID, created)
	})
	return out, err
}

// UpdateType applies a partial change (rename/reorder) and records the action.
func (s *Service) UpdateType(ctx context.Context, id string, patch domain.TypePatch) (domain.Type, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.Type{}, domain.ErrInvalid
	}
	var out domain.Type
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateType(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "rank.type.update", targetType, id, updated)
	})
	return out, err
}

// ---------------------------------------------------------------- ranks

// AddRank validates the request, ensures the parent type is an active leaf, validates the optional
// standardized grade, creates the rank, and records the action — all in one transaction. gradeCode nil
// = no cross-system grade; a non-nil gradeCode must exist in the rank_grades catalog (else
// ErrGradeNotFound).
func (s *Service) AddRank(ctx context.Context, typeID, code, name string, abbreviation, gradeCode *string, sortOrder *int) (domain.Rank, error) {
	r := domain.Rank{Code: code, Name: name, TypeID: typeID}
	if err := r.Validate(); err != nil {
		return domain.Rank{}, err
	}
	var out domain.Rank
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetType(ctx, typeID); err != nil {
			return err // ErrTypeNotFound when the parent is absent or soft-deleted
		}
		childCount, err := repo.CountActiveChildTypes(ctx, typeID)
		if err != nil {
			return err
		}
		if childCount > 0 {
			return errors.Join(domain.ErrInvalid, errors.New("type has child types; ranks live on leaf types only"))
		}
		if err := s.validateGrade(ctx, repo, gradeCode); err != nil {
			return err
		}
		created, err := repo.InsertRank(ctx, typeID, code, name, abbreviation, gradeCode, sortOrder)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "rank.rank.create", targetRank, created.ID, created)
	})
	return out, err
}

// UpdateRank applies a partial change (rename/reorder/abbreviation/grade) and records the action. A
// non-nil patch.GradeCode must exist in the rank_grades catalog.
func (s *Service) UpdateRank(ctx context.Context, id string, patch domain.RankPatch) (domain.Rank, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.Rank{}, domain.ErrInvalid
	}
	var out domain.Rank
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := s.validateGrade(ctx, repo, patch.GradeCode); err != nil {
			return err
		}
		updated, err := repo.UpdateRank(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "rank.rank.update", targetRank, id, updated)
	})
	return out, err
}

// validateGrade rejects an unknown standardized grade_code (ErrGradeNotFound); a nil/empty code is a
// no-op (the rank simply carries no cross-system grade).
func (s *Service) validateGrade(ctx context.Context, repo domain.Repository, gradeCode *string) error {
	if gradeCode == nil || *gradeCode == "" {
		return nil
	}
	if _, err := repo.GetGradeByCode(ctx, *gradeCode); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------- delete

// DeleteNode soft-deletes a scheme node at the given level, blocked if it is in use (a category with
// active types, a type with active ranks or active child types). A rank's in-use check (held by a person) lands with M5 —
// the person->rank FK is ON DELETE RESTRICT, so the directory stays consistent regardless. All in one
// transaction, with the action recorded.
func (s *Service) DeleteNode(ctx context.Context, level domain.Level, id string) error {
	if !domain.ValidLevel(level) {
		return domain.ErrInvalid
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		switch level {
		case domain.LevelSystem:
			if _, err := repo.GetSystem(ctx, id); err != nil {
				return err
			}
			count, err := repo.CountActiveCategories(ctx, id)
			if err != nil {
				return err
			}
			if count > 0 {
				return domain.ErrInUse // a system with active categories cannot be removed
			}
			if err := repo.SoftDeleteSystem(ctx, id); err != nil {
				return err
			}
			return s.record(ctx, tx, "rank.system.delete", targetSystem, id, map[string]string{"id": id})
		case domain.LevelCategory:
			if _, err := repo.GetCategory(ctx, id); err != nil {
				return err
			}
			count, err := repo.CountActiveTypes(ctx, id)
			if err != nil {
				return err
			}
			if count > 0 {
				return domain.ErrInUse
			}
			if err := repo.SoftDeleteCategory(ctx, id); err != nil {
				return err
			}
			return s.record(ctx, tx, "rank.category.delete", targetCategory, id, map[string]string{"id": id})
		case domain.LevelType:
			if _, err := repo.GetType(ctx, id); err != nil {
				return err
			}
			rankCount, err := repo.CountActiveRanks(ctx, id)
			if err != nil {
				return err
			}
			childCount, err := repo.CountActiveChildTypes(ctx, id)
			if err != nil {
				return err
			}
			if rankCount > 0 || childCount > 0 {
				return domain.ErrInUse // a type with active ranks or active child types is in use
			}
			if err := repo.SoftDeleteType(ctx, id); err != nil {
				return err
			}
			return s.record(ctx, tx, "rank.type.delete", targetType, id, map[string]string{"id": id})
		default: // domain.LevelRank
			if _, err := repo.GetRank(ctx, id); err != nil {
				return err
			}
			if err := repo.SoftDeleteRank(ctx, id); err != nil {
				return err
			}
			return s.record(ctx, tx, "rank.rank.delete", targetRank, id, map[string]string{"id": id})
		}
	})
}

// ---------------------------------------------------------------- helpers

// inTx runs fn in a transaction, committing on success and rolling back on error (the deferred
// rollback after a successful commit is a no-op).
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the audit
// entry commits iff the change commits (D-Audit). The actor is the interim system actor. Rank writes
// are instance-scoped, so no unit is attributed.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID composes the Action RID via the same SQL generator every module uses, so the audit
// log's action__<type> RID-shape CHECK is satisfied. The action code (e.g. "rank.type.create")
// becomes the entity_type slot "action__rank_type_create".
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	entityType := "action__" + sanitizeAction(action)
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_rid('rank', $1)", entityType).Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func sanitizeAction(action string) string {
	b := make([]byte, len(action))
	for i := 0; i < len(action); i++ {
		if action[i] == '.' {
			b[i] = '_'
		} else {
			b[i] = action[i]
		}
	}
	return string(b)
}

// requestID is the correlation key shared with logs/metrics/traces: the request's trace id, with a
// generated fallback for out-of-request callers (e.g. integration tests) so the audit Entry always
// has a non-empty requestId.
func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
