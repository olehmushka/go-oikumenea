// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package appkit holds the small transaction + audit plumbing shared by the person-family application
// services (person core, personprofile, personsensitive) after the R-09 split. It exists so the split
// modules reuse one copy of the "open a tx, do work, record an audit row in the same tx" pattern
// (D-Audit) instead of triplicating it. It depends only on the audit application service, the platform
// DB surface, and pgx — never on any person module, so every person-family module may import it without
// a cycle.
//
// Audit payloads for person-family writes carry only non-PII identifiers (ids, changed keys/status) —
// never names or personal data — because person is the primary PII store.
package appkit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// TargetPerson is the audited entity kind: every person-family action targets a person id.
const TargetPerson = "person"

// InTx runs fn inside a single transaction opened on pool, committing on success and rolling back on
// error (or panic-free early return). A write and its audit row commit together (D-Audit).
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Recorder wraps the audit application service to write person-family audit entries. Construct one per
// application service with NewRecorder(auditSvc).
type Recorder struct{ audit *auditapp.Service }

// NewRecorder binds a Recorder to the audit application service.
func NewRecorder(audit *auditapp.Service) *Recorder { return &Recorder{audit: audit} }

// Record writes a `system`-actor audit entry for a person-scoped action in the caller's transaction,
// minting the action RID and correlating it to the request. after is JSON-marshaled (non-PII only).
func (r *Recorder) Record(ctx context.Context, tx pgx.Tx, subsystem, action, targetID string, after any) error {
	rid, err := newActionRID(ctx, tx)
	if err != nil {
		return err
	}
	return r.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  subsystem,
		Action:     action,
		TargetType: TargetPerson,
		TargetID:   targetID,
		RequestID:  ReqID(ctx),
		After:      ToJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// newActionRID mints a person action RID (service 6, kind 3=action, type 0) in the caller's tx.
func newActionRID(ctx context.Context, tx pgx.Tx) (string, error) {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(6, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

// ReqID resolves the correlation id from the trace context, falling back to a generated one.
func ReqID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

// ToJSON marshals v to a JSON audit payload, returning nil on error.
func ToJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
