// Package application holds the rank module's application service — the orchestrator the transport
// layer calls to read/mutate the single system-wide rank scheme, recording an audit row in the same
// transaction as each write (D-Audit). It depends on the domain port, the platform DB surface, and
// the audit service; it never imports the adapters package directly (the repository factory is
// injected by module.go). Rank is a directory attribute and never an authorization input.
package application

import (
	"context"
	"encoding/json"

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

// Scheme is the whole rank scheme as three flat, ordered lists; the transport assembles the nested
// category -> type -> rank tree (and the localized names) for the response.
type Scheme struct {
	Categories []domain.Category
	Types      []domain.Type
	Ranks      []domain.Rank
}

// GetScheme reads the whole scheme in seniority order (each list ordered by sort_order, code).
func (s *Service) GetScheme(ctx context.Context) (Scheme, error) {
	repo := s.newRepo(s.pool)
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
	return Scheme{Categories: categories, Types: types, Ranks: ranks}, nil
}

// ---------------------------------------------------------------- categories

// AddCategory validates and creates a category, then records the action. sortOrder nil appends last.
func (s *Service) AddCategory(ctx context.Context, code, name string, sortOrder *int) (domain.Category, error) {
	c := domain.Category{Code: code, Name: name}
	if err := c.Validate(); err != nil {
		return domain.Category{}, err
	}
	var out domain.Category
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertCategory(ctx, code, name, sortOrder)
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

// AddType validates the request, ensures the parent category is active, creates the type, and records
// the action — all in one transaction.
func (s *Service) AddType(ctx context.Context, categoryID, code, name string, sortOrder *int) (domain.Type, error) {
	t := domain.Type{Code: code, Name: name, CategoryID: categoryID}
	if err := t.Validate(); err != nil {
		return domain.Type{}, err
	}
	var out domain.Type
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetCategory(ctx, categoryID); err != nil {
			return err // ErrCategoryNotFound when the parent is absent or soft-deleted
		}
		created, err := repo.InsertType(ctx, categoryID, code, name, sortOrder)
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

// AddRank validates the request, ensures the parent type is active, creates the rank, and records the
// action — all in one transaction.
func (s *Service) AddRank(ctx context.Context, typeID, code, name string, abbreviation *string, sortOrder *int) (domain.Rank, error) {
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
		created, err := repo.InsertRank(ctx, typeID, code, name, abbreviation, sortOrder)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "rank.rank.create", targetRank, created.ID, created)
	})
	return out, err
}

// UpdateRank applies a partial change (rename/reorder/abbreviation) and records the action.
func (s *Service) UpdateRank(ctx context.Context, id string, patch domain.RankPatch) (domain.Rank, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.Rank{}, domain.ErrInvalid
	}
	var out domain.Rank
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateRank(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "rank.rank.update", targetRank, id, updated)
	})
	return out, err
}

// ---------------------------------------------------------------- delete

// DeleteNode soft-deletes a scheme node at the given level, blocked if it is in use (a category with
// active types, a type with active ranks). A rank's in-use check (held by a person) lands with M5 —
// the person->rank FK is ON DELETE RESTRICT, so the directory stays consistent regardless. All in one
// transaction, with the action recorded.
func (s *Service) DeleteNode(ctx context.Context, level domain.Level, id string) error {
	if !domain.ValidLevel(level) {
		return domain.ErrInvalid
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		switch level {
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
			count, err := repo.CountActiveRanks(ctx, id)
			if err != nil {
				return err
			}
			if count > 0 {
				return domain.ErrInUse
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
