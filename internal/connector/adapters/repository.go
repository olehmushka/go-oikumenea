// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters is the connector-plane module's pgx-backed persistence adapter (M53 /
// D-ConnectorPlane). Raw pgx over a single command surface (the pool for reads, a tx for audited
// writes) — the externalorg/vehicle style. Postgres constraint violations (23505 unique) map to
// domain sentinels so the transport can render a readable 409 rather than a 500.
package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/connector/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the connector-plane persistence adapter bound to one command surface (pool or tx).
type Repository struct{ c db.DBTX }

// NewRepository binds a repository to the given command surface.
func NewRepository(conn db.DBTX) *Repository { return &Repository{c: conn} }

var _ domain.Repository = (*Repository)(nil)

// ---- small scan helpers ----

func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if t.Valid {
		v := t.Time
		return &v
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

const connectorCols = `id, code, name, description, principal_id, status, last_seen_at, created_at, updated_at`

func scanConnector(row pgx.Row) (domain.Connector, error) {
	var (
		c           domain.Connector
		description pgtype.Text
		principalID pgtype.Text
		lastSeenAt  pgtype.Timestamptz
	)
	if err := row.Scan(&c.ID, &c.Code, &c.Name, &description, &principalID, &c.Status, &lastSeenAt,
		&c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.Connector{}, err
	}
	c.Description = textVal(description)
	c.PrincipalID = textVal(principalID)
	c.LastSeenAt = timePtr(lastSeenAt)
	return c, nil
}

// ---- registry ----

// UpsertConnectorByPrincipal is the self-registration write. It keys on the CALLING principal, not on
// the code: an agent owns exactly one registry row, so re-registering under a changed code renames its
// row rather than creating a second one. If the code is already live under a DIFFERENT principal the
// partial-unique index fires and this returns ErrConflict — that is the anti-impersonation check.
func (r *Repository) UpsertConnectorByPrincipal(ctx context.Context, in domain.RegistrationInput) (domain.Connector, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.connector_connectors (code, name, description, principal_id, last_seen_at)
		VALUES ($1, $2, nullif($3,''), $4::uuid, now())
		ON CONFLICT (principal_id) WHERE deleted_at IS NULL AND principal_id IS NOT NULL
		DO UPDATE SET code         = EXCLUDED.code,
		              name         = EXCLUDED.name,
		              description  = EXCLUDED.description,
		              last_seen_at = now()
		RETURNING `+connectorCols, in.Code, in.Name, in.Description, in.PrincipalID)
	c, err := scanConnector(row)
	if isUniqueViolation(err) {
		return domain.Connector{}, domain.ErrConflict
	}
	return c, err
}

func (r *Repository) GetConnector(ctx context.Context, id string) (domain.Connector, error) {
	row := r.c.QueryRow(ctx, `
		SELECT `+connectorCols+`
		FROM oikumenea.connector_connectors
		WHERE id = $1::uuid AND deleted_at IS NULL`, id)
	c, err := scanConnector(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Connector{}, domain.ErrConnectorNotFound
	}
	return c, err
}

func (r *Repository) GetConnectorByPrincipal(ctx context.Context, principalID string) (domain.Connector, error) {
	row := r.c.QueryRow(ctx, `
		SELECT `+connectorCols+`
		FROM oikumenea.connector_connectors
		WHERE principal_id = $1::uuid AND deleted_at IS NULL`, principalID)
	c, err := scanConnector(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Connector{}, domain.ErrConnectorNotFound
	}
	return c, err
}

func (r *Repository) ListConnectors(ctx context.Context, after string, lim int) ([]domain.Connector, error) {
	rows, err := r.c.Query(ctx, `
		SELECT `+connectorCols+`
		FROM oikumenea.connector_connectors
		WHERE deleted_at IS NULL AND ($1 = '' OR id > $1::uuid)
		ORDER BY id LIMIT $2`, after, lim)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Connector
	for rows.Next() {
		c, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) TouchLastSeen(ctx context.Context, connectorID string) error {
	_, err := r.c.Exec(ctx, `
		UPDATE oikumenea.connector_connectors SET last_seen_at = now()
		WHERE id = $1::uuid AND deleted_at IS NULL`, connectorID)
	return err
}

// ---- sources ----

const sourceCols = `id, connector_id, code, name, object_type, schedule, enabled`

func scanSource(row pgx.Row) (domain.Source, error) {
	var (
		s          domain.Source
		objectType pgtype.Text
		schedule   pgtype.Text
	)
	if err := row.Scan(&s.ID, &s.ConnectorID, &s.Code, &s.Name, &objectType, &schedule, &s.Enabled); err != nil {
		return domain.Source{}, err
	}
	s.ObjectType = textVal(objectType)
	s.Schedule = textVal(schedule)
	return s, nil
}

// ReplaceSources makes the declared set authoritative for this connector: declared sources are
// upserted, and any live source NOT in the payload is soft-deleted. That is what lets a connector
// converge the registry by simply re-registering at boot, with no operator reconciling drift.
//
// Soft-delete rather than DELETE because sync_runs reference sources with ON DELETE CASCADE — a hard
// delete would silently destroy a source's run history when it is merely renamed or retired.
func (r *Repository) ReplaceSources(ctx context.Context, connectorID string, decls []domain.SourceDeclaration) ([]domain.Source, error) {
	codes := make([]string, 0, len(decls))
	for _, d := range decls {
		codes = append(codes, d.Code)
		if _, err := r.c.Exec(ctx, `
			INSERT INTO oikumenea.connector_sources (connector_id, code, name, object_type, schedule, enabled)
			VALUES ($1::uuid, $2, $3, nullif($4,''), nullif($5,''), $6)
			ON CONFLICT (connector_id, code) WHERE deleted_at IS NULL
			DO UPDATE SET name        = EXCLUDED.name,
			              object_type = EXCLUDED.object_type,
			              schedule    = EXCLUDED.schedule,
			              enabled     = EXCLUDED.enabled`,
			connectorID, d.Code, d.Name, d.ObjectType, d.Schedule, d.Enabled); err != nil {
			return nil, err
		}
	}
	// Retire whatever the connector no longer declares. `= ANY($2)` with an empty array retires all,
	// which is the correct reading of a registration that declares no sources.
	if _, err := r.c.Exec(ctx, `
		UPDATE oikumenea.connector_sources SET deleted_at = now()
		WHERE connector_id = $1::uuid AND deleted_at IS NULL AND NOT (code = ANY($2))`,
		connectorID, codes); err != nil {
		return nil, err
	}
	return r.ListSources(ctx, connectorID)
}

func (r *Repository) ListSources(ctx context.Context, connectorID string) ([]domain.Source, error) {
	rows, err := r.c.Query(ctx, `
		SELECT `+sourceCols+`
		FROM oikumenea.connector_sources
		WHERE connector_id = $1::uuid AND deleted_at IS NULL
		ORDER BY code`, connectorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) GetSourceByCode(ctx context.Context, connectorID, code string) (domain.Source, error) {
	row := r.c.QueryRow(ctx, `
		SELECT `+sourceCols+`
		FROM oikumenea.connector_sources
		WHERE connector_id = $1::uuid AND code = $2 AND deleted_at IS NULL`, connectorID, code)
	s, err := scanSource(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Source{}, domain.ErrSourceNotFound
	}
	return s, err
}

// ---- runs ----

const runCols = `id, source_id, external_run_id, state, created_count, updated_count, skipped_count,
	error, started_at, finished_at`

func scanRun(row pgx.Row) (domain.SyncRun, error) {
	var (
		r             domain.SyncRun
		externalRunID pgtype.Text
		errText       pgtype.Text
		finishedAt    pgtype.Timestamptz
	)
	if err := row.Scan(&r.ID, &r.SourceID, &externalRunID, &r.State, &r.Created, &r.Updated,
		&r.Skipped, &errText, &r.StartedAt, &finishedAt); err != nil {
		return domain.SyncRun{}, err
	}
	r.ExternalRunID = textVal(externalRunID)
	r.Error = textVal(errText)
	r.FinishedAt = timePtr(finishedAt)
	return r, nil
}

// UpsertRun records a report. Idempotent on (source, externalRunId) — a connector retrying its report
// (they retry with backoff) updates the run instead of duplicating it.
//
// A report WITHOUT an externalRunId cannot be deduplicated, so it always inserts. That is the honest
// behaviour: without a connector-side identifier there is nothing to be idempotent on.
func (r *Repository) UpsertRun(ctx context.Context, sourceID string, in domain.ReportInput) (domain.SyncRun, error) {
	started := time.Now()
	if in.StartedAt != nil {
		started = *in.StartedAt
	}
	if in.ExternalRunID == "" {
		row := r.c.QueryRow(ctx, `
			INSERT INTO oikumenea.connector_sync_runs
			  (source_id, state, created_count, updated_count, skipped_count, error, started_at, finished_at)
			VALUES ($1::uuid, $2, $3, $4, $5, nullif($6,''), $7, $8)
			RETURNING `+runCols,
			sourceID, in.State, in.Created, in.Updated, in.Skipped, in.Error, started, in.FinishedAt)
		return scanRun(row)
	}
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.connector_sync_runs
		  (source_id, external_run_id, state, created_count, updated_count, skipped_count, error, started_at, finished_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, nullif($7,''), $8, $9)
		ON CONFLICT (source_id, external_run_id) WHERE external_run_id IS NOT NULL
		DO UPDATE SET state         = EXCLUDED.state,
		              created_count = EXCLUDED.created_count,
		              updated_count = EXCLUDED.updated_count,
		              skipped_count = EXCLUDED.skipped_count,
		              error         = EXCLUDED.error,
		              finished_at   = EXCLUDED.finished_at
		RETURNING `+runCols,
		sourceID, in.ExternalRunID, in.State, in.Created, in.Updated, in.Skipped, in.Error,
		started, in.FinishedAt)
	return scanRun(row)
}

// ListRuns returns runs newest-first. Keyset pages on id DESC (RIDs are uuidv8 with a time-ordered
// prefix, so id DESC and started_at DESC agree), matching the connector_sync_runs_source_started_idx.
func (r *Repository) ListRuns(ctx context.Context, sourceID, after string, lim int) ([]domain.SyncRun, error) {
	rows, err := r.c.Query(ctx, `
		SELECT `+runCols+`
		FROM oikumenea.connector_sync_runs
		WHERE ($1 = '' OR source_id = $1::uuid)
		  AND ($2 = '' OR id < $2::uuid)
		ORDER BY id DESC LIMIT $3`, sourceID, after, lim)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SyncRun
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}
