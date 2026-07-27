// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application is the connector-plane orchestrator (M53 / D-ConnectorPlane): audited
// self-registration and sync-run reporting by machine subjects, plus the operator read surfaces.
// Every write runs in a transaction that also records the audit Action (D-Audit); reads run on the pool.
//
// Reports are recorded under a `system` actor carrying `ActorPrincipalID` — the M51 machine-actor shape
// (audit_log.actor_principal_id). That is what makes "which agent told us this" answerable after the
// fact, and it is why the principal is taken from the request context rather than the wire.
//
// There is deliberately NO Trigger/Retry/Pause here: the core records what connectors report and never
// orchestrates them (D-ConnectorPlane rejects core-side orchestration).
package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/connector/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

const (
	auditSubsystem  = "connector-plane"
	defaultPageSize = 50
	maxPageSize     = 200
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, tx for writes).
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the connector-plane application service.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
}

// NewService wires the service with the pool, repository factory, and audit service.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// ============================ self-service (machine subjects) ============================

// Register is a connector's idempotent self-description. `principalID` comes from the request context
// (the authenticated machine subject) — never from the request body — so a connector cannot register
// as another. The declared source set becomes authoritative for this connector.
func (s *Service) Register(ctx context.Context, in domain.RegistrationInput) (domain.Connector, []domain.Source, error) {
	if err := in.Validate(); err != nil {
		return domain.Connector{}, nil, err
	}
	var (
		c    domain.Connector
		srcs []domain.Source
	)
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		var err error
		if c, err = repo.UpsertConnectorByPrincipal(ctx, in); err != nil {
			return err
		}
		if srcs, err = repo.ReplaceSources(ctx, c.ID, in.Sources); err != nil {
			return err
		}
		return s.record(ctx, tx, "connector.register", "connector", c.ID, in.PrincipalID, c)
	})
	if err != nil {
		return domain.Connector{}, nil, err
	}
	return c, srcs, nil
}

// Report records one sync run for a source of the CALLING connector, resolving both from the
// principal. Idempotent on (source, externalRunId), so a connector retrying its report converges.
func (s *Service) Report(ctx context.Context, in domain.ReportInput) (domain.SyncRun, error) {
	if err := in.Validate(); err != nil {
		return domain.SyncRun{}, err
	}
	var run domain.SyncRun
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		c, err := repo.GetConnectorByPrincipal(ctx, in.PrincipalID)
		if err != nil {
			return err
		}
		src, err := repo.GetSourceByCode(ctx, c.ID, in.SourceCode)
		if err != nil {
			return err
		}
		if run, err = repo.UpsertRun(ctx, src.ID, in); err != nil {
			return err
		}
		if err := repo.TouchLastSeen(ctx, c.ID); err != nil {
			return err
		}
		return s.record(ctx, tx, "connector.sync-run."+run.State, "sync_run", run.ID, in.PrincipalID, run)
	})
	if err != nil {
		return domain.SyncRun{}, err
	}
	return run, nil
}

// ============================ operator reads ============================

func (s *Service) ListConnectors(ctx context.Context, after string, lim int) ([]domain.Connector, error) {
	return s.newRepo(s.querier(ctx)).ListConnectors(ctx, after, pageSize(lim))
}

func (s *Service) GetConnector(ctx context.Context, id string) (domain.Connector, error) {
	return s.newRepo(s.querier(ctx)).GetConnector(ctx, id)
}

func (s *Service) ListSources(ctx context.Context, connectorID string) ([]domain.Source, error) {
	repo := s.newRepo(s.querier(ctx))
	// Resolve the connector first so an unknown id is a 404 rather than an empty list — an empty list
	// would read as "this connector declares nothing", which is a different fact.
	if _, err := repo.GetConnector(ctx, connectorID); err != nil {
		return nil, err
	}
	return repo.ListSources(ctx, connectorID)
}

func (s *Service) ListRuns(ctx context.Context, sourceID, after string, lim int) ([]domain.SyncRun, error) {
	return s.newRepo(s.querier(ctx)).ListRuns(ctx, sourceID, after, pageSize(lim))
}

// ============================ self (wiring.cursor.read) ============================

// GetConnectorByPrincipal returns the connector a machine subject registered as itself (the M53 wiring
// `self` primitive). Unregistered → domain.ErrConnectorNotFound.
func (s *Service) GetConnectorByPrincipal(ctx context.Context, principalID string) (domain.Connector, error) {
	return s.newRepo(s.querier(ctx)).GetConnectorByPrincipal(ctx, principalID)
}

// LatestRun returns the most recent sync run for a source, or (zero, false) if it has never run. This
// is a source's "cursor" — where a connector last got to (D-ConnectorPlane pull-wiring).
func (s *Service) LatestRun(ctx context.Context, sourceID string) (domain.SyncRun, bool, error) {
	runs, err := s.newRepo(s.querier(ctx)).ListRuns(ctx, sourceID, "", 1)
	if err != nil {
		return domain.SyncRun{}, false, err
	}
	if len(runs) == 0 {
		return domain.SyncRun{}, false, nil
	}
	return runs[0], true, nil
}

// ============================ helpers ============================

func pageSize(n int) int {
	if n <= 0 {
		return defaultPageSize
	}
	if n > maxPageSize {
		return maxPageSize
	}
	return n
}

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

// record mints an Action RID (connector service=20, kind=action=3, type=0) in the caller's transaction
// and writes the audit row on it (D-Audit). The actor is `system` + the reporting principal — the M51
// machine-actor shape — so an operator can answer "which agent claimed this run?" later.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID, principalID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(20, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:               rid,
		ActorType:        auditdomain.ActorSystem,
		ActorPrincipalID: principalID,
		Subsystem:        auditSubsystem,
		Action:           action,
		TargetType:       targetType,
		TargetID:         targetID,
		RequestID:        requestID(ctx),
		After:            toJSON(after),
		Outcome:          auditdomain.OutcomeSuccess,
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
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
