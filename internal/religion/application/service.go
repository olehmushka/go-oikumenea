// Package application is the religion module's orchestrator (D-Religion, M22): audited writes, the
// recursive taxonomy + maintained closure (rebuilt on every taxon insert/reparent/delete, mirroring
// tenant/education), the nearest-declared-wins theism resolution, and the per-unit organization
// attributes. Every write runs in a transaction that also records the audit Action (D-Audit); reads run
// on the pool. Taxonomy/catalog writes are recorded as a `system` action (instance reference data);
// per-unit org writes attribute the unit. createChildOrg reuses the tenant service to build the
// canonical-graph governance tree and enforces the excludes_child_creation policy.
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
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "religion-admin"
	defaultPageSize = 50
	maxPageSize     = 200
	canonicalGraph  = "canonical"
)

// Repo is the persistence surface the service needs (consumer-defined; *adapters.Repository satisfies it).
type Repo interface {
	ListTaxonRanks(ctx context.Context) ([]domain.TaxonRank, error)
	UpsertTaxonRank(ctx context.Context, code, name string, ordinal int, sortOrder *int) (domain.TaxonRank, error)
	ListClassifications(ctx context.Context) ([]domain.Classification, error)
	GetClassificationsByIDs(ctx context.Context, ids []string) ([]domain.Classification, error)
	UpsertClassification(ctx context.Context, code, name string, description *string, sortOrder *int) (domain.Classification, error)
	ListOrgKinds(ctx context.Context) ([]domain.OrgKind, error)
	UpsertOrgKind(ctx context.Context, code, name string, religionID *string, ordinal, sortOrder *int) (domain.OrgKind, error)
	ListPolicyKinds(ctx context.Context) ([]domain.PolicyKind, error)
	UpsertPolicyKind(ctx context.Context, code, name string, description *string, sortOrder *int) (domain.PolicyKind, error)
	GetTaxon(ctx context.Context, id string) (domain.Taxon, error)
	InsertTaxon(ctx context.Context, in domain.TaxonInput) (domain.Taxon, error)
	UpdateTaxon(ctx context.Context, id string, up domain.TaxonUpdate) (domain.Taxon, error)
	SoftDeleteTaxon(ctx context.Context, id string) error
	CountTaxonChildren(ctx context.Context, id string) (int, error)
	CountUnitsClassifiedBy(ctx context.Context, taxonID string) (int, error)
	IsDescendant(ctx context.Context, ancestorID, candidateID string) (bool, error)
	SetTaxonParent(ctx context.Context, id, parentID string) error
	ListTaxa(ctx context.Context, rank, parent, religion, query, after string, limit int) ([]domain.Taxon, error)
	RebuildClosure(ctx context.Context) (domain.ClosureReport, error)
	EffectiveClassificationsForTaxon(ctx context.Context, taxonID string) ([]domain.Classification, error)
	SetTaxonClassifications(ctx context.Context, taxonID string, classificationIDs []string) error
	GetOrgProfileRow(ctx context.Context, unitID string) (domain.OrgProfile, error)
	UpsertOrgProfile(ctx context.Context, unitID string, orgKindID, shortCode *string) (domain.OrgProfile, error)
	ListOrgClassifications(ctx context.Context, unitID string) ([]domain.OrgClassification, error)
	ClearPrimaryClassification(ctx context.Context, unitID string) error
	AddOrgClassification(ctx context.Context, unitID, taxonID string, isPrimary bool, source, confidence *string) (domain.OrgClassification, error)
	RemoveOrgClassification(ctx context.Context, unitID, linkID string) error
	SetUnitClassifications(ctx context.Context, unitID string, classificationIDs []string) error
	EffectiveTypeForUnit(ctx context.Context, unitID string) (domain.EffectiveType, error)
	ListOrgPolicies(ctx context.Context, unitID string) ([]domain.OrgPolicy, error)
	AddOrgPolicy(ctx context.Context, unitID, policyKindID string, reason, decidedByPersonID *string) (domain.OrgPolicy, error)
	RemoveOrgPolicy(ctx context.Context, unitID, policyID string) error
	HasActivePolicy(ctx context.Context, unitID, policyKindCode string) (bool, error)
	// clergy (M23)
	ListGradeCategories(ctx context.Context) ([]domain.GradeCategory, error)
	UpsertGradeCategory(ctx context.Context, traditionTaxonID *string, code, name string, ordinal, sortOrder *int) (domain.GradeCategory, error)
	ListClergyGrades(ctx context.Context, tradition string) ([]domain.ClergyGrade, error)
	UpsertClergyGrade(ctx context.Context, traditionTaxonID *string, gradeCategoryID, code, name string, ordinal int, sortOrder *int) (domain.ClergyGrade, error)
	ListOfficeTypes(ctx context.Context) ([]domain.OfficeType, error)
	UpsertOfficeType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.OfficeType, error)
	ListClergyCredentialsByPerson(ctx context.Context, personID string) ([]domain.ClergyCredential, error)
	ListClergyCredentialsByUnit(ctx context.Context, unitID string) ([]domain.ClergyCredential, error)
	GetClergyCredential(ctx context.Context, id string) (domain.ClergyCredential, error)
	InsertClergyCredential(ctx context.Context, in domain.ClergyCredentialInput) (domain.ClergyCredential, error)
	UpdateClergyCredential(ctx context.Context, id string, up domain.ClergyCredentialUpdate) (domain.ClergyCredential, error)
	// lay affiliation (M24)
	ListAffiliationTypes(ctx context.Context) ([]domain.AffiliationType, error)
	UpsertAffiliationType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.AffiliationType, error)
	ListAffiliationsByPerson(ctx context.Context, personID string) ([]domain.StoredAffiliation, error)
	GetAffiliation(ctx context.Context, id string) (domain.StoredAffiliation, error)
	InsertAffiliation(ctx context.Context, in domain.AffiliationInput) (domain.StoredAffiliation, error)
	UpdateAffiliation(ctx context.Context, id string, up domain.AffiliationUpdate) (domain.StoredAffiliation, error)
	SoftDeleteAffiliation(ctx context.Context, id string) error
	CryptoEraseAffiliations(ctx context.Context, personID string) (int64, error)
}

// RepositoryFactory binds a Repo to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) Repo

// Service is the religion application service.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	tenant  *tenantapp.Service
	cipher  *crypto.Cipher // envelope cipher for pii:special affiliation values (D-SpecialPII, M24)
}

// NewService wires the service with the pool, repository factory, audit, tenant services, and the
// envelope cipher (D-SpecialPII — used to seal/open lay-affiliation belief values).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, tenant *tenantapp.Service, cipher *crypto.Cipher) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, tenant: tenant, cipher: cipher}
}

// ============================ catalogs ============================

func (s *Service) ListTaxonRanks(ctx context.Context) ([]domain.TaxonRank, error) {
	return s.newRepo(s.querier(ctx)).ListTaxonRanks(ctx)
}

func (s *Service) UpsertTaxonRank(ctx context.Context, code, name string, ordinal int, sortOrder *int) (domain.TaxonRank, error) {
	var out domain.TaxonRank
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertTaxonRank(ctx, code, name, ordinal, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.taxon-rank.upsert", v.ID, "", v)
	})
	return out, err
}

func (s *Service) ListClassifications(ctx context.Context) ([]domain.Classification, error) {
	return s.newRepo(s.querier(ctx)).ListClassifications(ctx)
}

func (s *Service) UpsertClassification(ctx context.Context, code, name string, description *string, sortOrder *int) (domain.Classification, error) {
	var out domain.Classification
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertClassification(ctx, code, name, description, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.classification.upsert", v.ID, "", v)
	})
	return out, err
}

func (s *Service) ListOrgKinds(ctx context.Context) ([]domain.OrgKind, error) {
	return s.newRepo(s.querier(ctx)).ListOrgKinds(ctx)
}

func (s *Service) UpsertOrgKind(ctx context.Context, code, name string, religionID *string, ordinal, sortOrder *int) (domain.OrgKind, error) {
	var out domain.OrgKind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertOrgKind(ctx, code, name, religionID, ordinal, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.org-kind.upsert", v.ID, "", v)
	})
	return out, err
}

func (s *Service) ListPolicyKinds(ctx context.Context) ([]domain.PolicyKind, error) {
	return s.newRepo(s.querier(ctx)).ListPolicyKinds(ctx)
}

func (s *Service) UpsertPolicyKind(ctx context.Context, code, name string, description *string, sortOrder *int) (domain.PolicyKind, error) {
	var out domain.PolicyKind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertPolicyKind(ctx, code, name, description, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.policy-kind.upsert", v.ID, "", v)
	})
	return out, err
}

// ============================ taxonomy ============================

func (s *Service) GetTaxon(ctx context.Context, id string) (domain.Taxon, error) {
	return s.newRepo(s.querier(ctx)).GetTaxon(ctx, id)
}

func (s *Service) ListTaxa(ctx context.Context, rank, parent, religion, query, after string, pageSize int) ([]domain.Taxon, error) {
	return s.newRepo(s.querier(ctx)).ListTaxa(ctx, rank, parent, religion, query, after, clampPageSize(pageSize)+1)
}

func (s *Service) CreateTaxon(ctx context.Context, in domain.TaxonInput) (domain.Taxon, error) {
	if err := in.Validate(); err != nil {
		return domain.Taxon{}, err
	}
	var out domain.Taxon
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		created, err := repo.InsertTaxon(ctx, in)
		if err != nil {
			return err
		}
		if _, err := repo.RebuildClosure(ctx); err != nil {
			return err
		}
		out, err = repo.GetTaxon(ctx, created.ID)
		if err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.taxon.create", out.ID, "", out)
	})
	return out, err
}

func (s *Service) UpdateTaxon(ctx context.Context, id string, up domain.TaxonUpdate) (domain.Taxon, error) {
	var out domain.Taxon
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpdateTaxon(ctx, id, up)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.taxon.update", id, "", v)
	})
	return out, err
}

func (s *Service) DeleteTaxon(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		n, err := repo.CountTaxonChildren(ctx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return domain.ErrInUse
		}
		used, err := repo.CountUnitsClassifiedBy(ctx, id)
		if err != nil {
			return err
		}
		if used > 0 {
			return domain.ErrInUse
		}
		if err := repo.SoftDeleteTaxon(ctx, id); err != nil {
			return err
		}
		if _, err := repo.RebuildClosure(ctx); err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.taxon.delete", id, "", nil)
	})
}

func (s *Service) ReparentTaxon(ctx context.Context, id, parentID string) (domain.Taxon, error) {
	var out domain.Taxon
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetTaxon(ctx, id); err != nil {
			return err
		}
		if parentID != "" {
			if parentID == id {
				return domain.ErrTaxonCycle
			}
			// a cycle would result if the new parent is the taxon itself or one of its descendants
			isDesc, err := repo.IsDescendant(ctx, id, parentID)
			if err != nil {
				return err
			}
			if isDesc {
				return domain.ErrTaxonCycle
			}
		}
		if err := repo.SetTaxonParent(ctx, id, parentID); err != nil {
			return err
		}
		if _, err := repo.RebuildClosure(ctx); err != nil {
			return err
		}
		v, err := repo.GetTaxon(ctx, id)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.taxon.reparent", id, "", v)
	})
	return out, err
}

func (s *Service) RebuildClosure(ctx context.Context) (domain.ClosureReport, error) {
	var out domain.ClosureReport
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		rep, err := s.newRepo(tx).RebuildClosure(ctx)
		if err != nil {
			return err
		}
		out = rep
		return s.record(ctx, tx, "religion.taxonomy.rebuild-closure", "", "", rep)
	})
	return out, err
}

func (s *Service) EffectiveClassifications(ctx context.Context, taxonID string) ([]domain.Classification, error) {
	if _, err := s.newRepo(s.querier(ctx)).GetTaxon(ctx, taxonID); err != nil {
		return nil, err
	}
	return s.newRepo(s.querier(ctx)).EffectiveClassificationsForTaxon(ctx, taxonID)
}

func (s *Service) SetTaxonClassifications(ctx context.Context, taxonID string, classificationIDs []string) ([]domain.Classification, error) {
	var out []domain.Classification
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetTaxon(ctx, taxonID); err != nil {
			return err
		}
		if err := repo.SetTaxonClassifications(ctx, taxonID, classificationIDs); err != nil {
			return err
		}
		cls, err := repo.GetClassificationsByIDs(ctx, classificationIDs)
		if err != nil {
			return err
		}
		out = cls
		return s.record(ctx, tx, "religion.taxon.set-classifications", taxonID, "", classificationIDs)
	})
	return out, err
}

// ============================ organization ============================

func (s *Service) GetOrgProfile(ctx context.Context, unitID string) (domain.OrgProfile, error) {
	repo := s.newRepo(s.querier(ctx))
	p, err := repo.GetOrgProfileRow(ctx, unitID)
	if err != nil {
		return domain.OrgProfile{}, err
	}
	p.Classifications, err = repo.ListOrgClassifications(ctx, unitID)
	return p, err
}

func (s *Service) SetOrgProfile(ctx context.Context, unitID string, orgKindID, shortCode *string) (domain.OrgProfile, error) {
	var out domain.OrgProfile
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		p, err := repo.UpsertOrgProfile(ctx, unitID, orgKindID, shortCode)
		if err != nil {
			return err
		}
		p.Classifications, err = repo.ListOrgClassifications(ctx, unitID)
		if err != nil {
			return err
		}
		out = p
		return s.record(ctx, tx, "religion.org-profile.set", unitID, unitID, p)
	})
	return out, err
}

func (s *Service) AddOrgClassification(ctx context.Context, unitID, taxonID string, isPrimary bool, source, confidence *string) (domain.OrgClassification, error) {
	var out domain.OrgClassification
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if isPrimary {
			if err := repo.ClearPrimaryClassification(ctx, unitID); err != nil {
				return err
			}
		}
		c, err := repo.AddOrgClassification(ctx, unitID, taxonID, isPrimary, source, confidence)
		if err != nil {
			return err
		}
		out = c
		return s.record(ctx, tx, "religion.org-classification.add", c.ID, unitID, c)
	})
	return out, err
}

func (s *Service) RemoveOrgClassification(ctx context.Context, unitID, linkID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).RemoveOrgClassification(ctx, unitID, linkID); err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.org-classification.remove", linkID, unitID, nil)
	})
}

func (s *Service) SetUnitTypeOverride(ctx context.Context, unitID string, classificationIDs []string) ([]domain.Classification, error) {
	var out []domain.Classification
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.SetUnitClassifications(ctx, unitID, classificationIDs); err != nil {
			return err
		}
		cls, err := repo.GetClassificationsByIDs(ctx, classificationIDs)
		if err != nil {
			return err
		}
		out = cls
		return s.record(ctx, tx, "religion.unit-type-override.set", unitID, unitID, classificationIDs)
	})
	return out, err
}

func (s *Service) EffectiveType(ctx context.Context, unitID string) (domain.EffectiveType, error) {
	return s.newRepo(s.querier(ctx)).EffectiveTypeForUnit(ctx, unitID)
}

func (s *Service) ListOrgPolicies(ctx context.Context, unitID string) ([]domain.OrgPolicy, error) {
	return s.newRepo(s.querier(ctx)).ListOrgPolicies(ctx, unitID)
}

func (s *Service) AddOrgPolicy(ctx context.Context, unitID, policyKindID string, reason, decidedByPersonID *string) (domain.OrgPolicy, error) {
	var out domain.OrgPolicy
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		p, err := s.newRepo(tx).AddOrgPolicy(ctx, unitID, policyKindID, reason, decidedByPersonID)
		if err != nil {
			return err
		}
		out = p
		return s.record(ctx, tx, "religion.org-policy.add", p.ID, unitID, p)
	})
	return out, err
}

func (s *Service) RemoveOrgPolicy(ctx context.Context, unitID, policyID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).RemoveOrgPolicy(ctx, unitID, policyID); err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.org-policy.remove", policyID, unitID, nil)
	})
}

// CreateChildOrg builds a child religious-body unit under parentUnitID in the canonical graph (a tenant
// unit + the canonical parent→child edge + the child's profile + an optional primary classification),
// rejecting it if the parent carries an active excludes_child_creation policy. The tenant operations run
// in their own transactions (D-Hexagonal cross-module mutation), so this is sequential, not atomic.
func (s *Service) CreateChildOrg(ctx context.Context, parentUnitID, code, name, visibility, orgKindID, primaryTaxonID string) (domain.OrgProfile, error) {
	excluded, err := s.newRepo(s.querier(ctx)).HasActivePolicy(ctx, parentUnitID, domain.PolicyExcludesChildCreation)
	if err != nil {
		return domain.OrgProfile{}, err
	}
	if excluded {
		return domain.OrgProfile{}, domain.ErrChildCreationExcluded
	}
	vis := tenantdomain.VisibilityPublic
	if visibility == string(tenantdomain.VisibilityShadow) {
		vis = tenantdomain.VisibilityShadow
	}
	child, err := s.tenant.CreateUnit(ctx, tenantdomain.Unit{Code: code, Name: name, Visibility: vis})
	if err != nil {
		return domain.OrgProfile{}, err
	}
	if _, err := s.tenant.AddEdge(ctx, child.ID, parentUnitID, canonicalGraph); err != nil {
		return domain.OrgProfile{}, err
	}
	var kindPtr *string
	if orgKindID != "" {
		kindPtr = &orgKindID
	}
	if _, err := s.SetOrgProfile(ctx, child.ID, kindPtr, nil); err != nil {
		return domain.OrgProfile{}, err
	}
	if primaryTaxonID != "" {
		if _, err := s.AddOrgClassification(ctx, child.ID, primaryTaxonID, true, nil, nil); err != nil {
			return domain.OrgProfile{}, err
		}
	}
	return s.GetOrgProfile(ctx, child.ID)
}

// ============================ helpers ============================

func clampPageSize(n int) int {
	if n <= 0 {
		return defaultPageSize
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return n
}

// querier returns the request-pinned RLS connection if one is in context (db.AcquireScoped/WithConn),
// else the bare pool. Reads/writes on the unit-scoped religion tables (org profiles/classifications/
// policies + clergy credentials) MUST go through it so the app.* RLS GUCs apply (D-RLSDefenseInDepth).
func (s *Service) querier(ctx context.Context) db.Querier {
	if c, ok := db.ConnFromContext(ctx); ok {
		return c
	}
	return s.pool
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.querier(ctx).Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID (religion service=16, kind=action=3, type=0) in the caller's transaction
// and writes the audit row on it (D-Audit). unitID is "" for instance-level taxonomy/catalog actions.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID, unitID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(16, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: "religion",
		TargetID:   targetID,
		UnitID:     unitID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
