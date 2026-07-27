// Package application is the external-organizations module's orchestrator (D-ExternalOrgs, M30): audited
// writes over the kind catalog + the external-organization node-space, with provisional/resolved
// resolution. Every write runs in a transaction that also records the audit Action (D-Audit); reads run
// on the pool. External orgs are instance-global external reference data, so writes are recorded under a
// `system` actor (mirroring vehicle/company).
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
	"github.com/olegamysk/go-oikumenea/internal/externalorg/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "external-org-admin"
	defaultPageSize = 50
	maxPageSize     = 200
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the external-organizations application service.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
}

// NewService wires the service with the pool, repository factory, and audit service.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// ============================ kind catalog ============================

func (s *Service) ListKinds(ctx context.Context) ([]domain.Kind, error) {
	return s.newRepo(s.querier(ctx)).ListKinds(ctx)
}

func (s *Service) UpsertKind(ctx context.Context, code, name string, sortOrder *int) (domain.Kind, error) {
	var out domain.Kind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		k, err := s.newRepo(tx).UpsertKind(ctx, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = k
		return s.record(ctx, tx, "external-org.kind.upsert", k.ID, k)
	})
	return out, err
}

func (s *Service) KindNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).KindNamesByIDs(ctx, ids)
}

func (s *Service) CountryNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return s.newRepo(s.querier(ctx)).CountryNamesByIDs(ctx, ids)
}

// ============================ organizations ============================

func (s *Service) GetOrg(ctx context.Context, id string) (domain.Organization, error) {
	o, err := s.newRepo(s.querier(ctx)).GetOrg(ctx, id)
	return o, mapNotFound(err, domain.ErrNotFound)
}

func (s *Service) ListOrgs(ctx context.Context, query, kindCode, countryID, status, after string, pageSize int) ([]domain.Organization, error) {
	return s.newRepo(s.querier(ctx)).ListOrgs(ctx, query, kindCode, countryID, status, after, clampPageSize(pageSize)+1)
}

func (s *Service) CreateOrg(ctx context.Context, in domain.OrgInput) (domain.Organization, error) {
	if err := in.Validate(); err != nil {
		return domain.Organization{}, err
	}
	var out domain.Organization
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetKind(ctx, in.KindID); err != nil {
			return mapNotFound(err, domain.ErrInvalid)
		}
		created, err := repo.InsertOrg(ctx, in)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "external-org.create", created.ID, created)
	})
	return out, err
}

func (s *Service) UpdateOrg(ctx context.Context, id string, up domain.OrgUpdate) (domain.Organization, error) {
	if err := validateUpdate(up); err != nil {
		return domain.Organization{}, err
	}
	var out domain.Organization
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		o, err := s.newRepo(tx).UpdateOrg(ctx, id, up)
		if err != nil {
			return mapNotFound(err, domain.ErrNotFound)
		}
		out = o
		return s.record(ctx, tx, "external-org.update", id, o)
	})
	return out, err
}

func (s *Service) DeleteOrg(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		n, err := s.newRepo(tx).SoftDeleteOrg(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return s.record(ctx, tx, "external-org.delete", id, nil)
	})
}

// MergeOrg resolves a provisional stub (fromID) into a canonical organization (intoID): it validates
// that fromID is provisional and intoID is a distinct, non-provisional, live org, then tombstones the
// stub. No edge-consumers exist yet (M33 institutional ties not built), so there is nothing to re-home;
// when they land, this is where a `ExternalOrgMerged` event would be published in the same transaction.
func (s *Service) MergeOrg(ctx context.Context, fromID, intoID, confidence string) (domain.Organization, error) {
	if fromID == intoID {
		return domain.Organization{}, domain.ErrInvalid
	}
	if confidence != "" && !domain.ValidConfidence(confidence) {
		return domain.Organization{}, domain.ErrInvalid
	}
	var out domain.Organization
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		from, err := repo.GetOrg(ctx, fromID)
		if err != nil {
			return mapNotFound(err, domain.ErrNotFound)
		}
		into, err := repo.GetOrg(ctx, intoID)
		if err != nil {
			return mapNotFound(err, domain.ErrNotFound)
		}
		if from.Status != domain.StatusProvisional || into.Status == domain.StatusProvisional {
			return domain.ErrInvalid
		}
		if _, err := repo.TombstoneOrg(ctx, fromID); err != nil {
			return err
		}
		out = into
		conf := confidence
		if conf == "" {
			conf = domain.ConfidencePossible
		}
		return s.record(ctx, tx, "external-org.merge", intoID, map[string]string{
			"id": intoID, "mergedFrom": fromID, "confidence": conf,
		})
	})
	return out, err
}

// ============================ infra ============================

func validateUpdate(up domain.OrgUpdate) error {
	if up.Status != nil && !domain.ValidStatus(*up.Status) {
		return domain.ErrInvalid
	}
	if up.Source != nil && !domain.ValidSource(*up.Source) {
		return domain.ErrInvalid
	}
	if up.Confidence != nil && !domain.ValidConfidence(*up.Confidence) {
		return domain.ErrInvalid
	}
	return nil
}

// pageSizePolicy is this module's page-size policy, clamped through the shared kernel (M56 /
// pkg/listing) instead of a local copy.
var pageSizePolicy = listing.PageSize{Default: defaultPageSize, Max: maxPageSize}

func clampPageSize(n int) int { return pageSizePolicy.Resolve(n) }

func (s *Service) querier(ctx context.Context) db.Querier {
	return db.RequestQuerier(ctx, s.pool)
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

// record mints an Action RID (external_organization service=18, kind=action=3, type=0) in the caller's
// transaction and writes the audit row on it (D-Audit). External-org data is instance-global reference
// data → system actor.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(18, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: "external_organization",
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

func mapNotFound(err error, sentinel error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sentinel
	}
	return err
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
