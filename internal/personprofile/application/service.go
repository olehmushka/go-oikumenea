// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application holds the personprofile module's application service — the orchestrator for the
// person directory's non-encrypted, person-owned directory data (R-09 split): citizenships, residences,
// addresses, the contact channels (email/phone/call-sign/messenger/social), the SPEAKS languages, the
// person↔person relationships, and the non-encrypted institutional ties (government positions, lobbying,
// external references).
//
// It shares the person aggregate root's domain kernel (internal/person/domain) and the transaction+audit
// plumbing (internal/person/appkit); it does not import the person core application or adapters packages
// beyond the composition seam. Audit payloads carry only non-PII identifiers (D-Audit).
package application

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	"github.com/olehmushka/go-oikumenea/internal/person/appkit"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// auditSubsystem labels the interim system actor for personprofile admin writes (mirrors the person core
// subsystem — these are still person-scoped admin actions).
const auditSubsystem = "person-admin"

// RepositoryFactory binds a domain.ProfileRepository to a command surface — the pool for reads, or a caller's
// transaction for an audited write (D-Audit). Injected by module.go. For PR-2a the factory returns the
// unified person adapters repository (the repository/query split lands in PR-2b).
type RepositoryFactory func(conn db.DBTX) domain.ProfileRepository

// Service is the personprofile application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly.
type Service struct {
	pool      *pgxpool.Pool
	newRepo   RepositoryFactory
	rec       *appkit.Recorder
	now       func() time.Time
	locations domain.LocationLookup // late-bound location catalog (D-PersonAddresses, M32): address FK check
}

// NewService wires the service with the pool, the repository factory, and the audit service. The location
// lookup seam is late-bound (SetLocationLookup).
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, rec: appkit.NewRecorder(audit), now: func() time.Time { return time.Now().UTC() }}
}

// SetLocationLookup binds the cross-module location query seam (D-PersonAddresses, M32) used to verify an
// address's location_id exists before writing. Late-bound at composition time (geo is built after
// person); when unset (e.g. tests that don't exercise addresses), the DB FK is the only backstop.
func (s *Service) SetLocationLookup(l domain.LocationLookup) { s.locations = l }

// MustBeBound reports whether the mandatory cross-module seams are wired (review-2026-07 R-11): the
// composition root calls it at boot so a forgotten setter fails startup instead of a request-time nil
// deref.
func (s *Service) MustBeBound() error {
	if s.locations == nil {
		return errors.New("personprofile service: location lookup seam not bound (SetLocationLookup)")
	}
	return nil
}

// inTx runs fn in a single transaction on the pool (a write and its audit row commit together).
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return appkit.InTx(ctx, s.pool, fn)
}

// record writes a person-admin audit entry in the caller's transaction (non-PII payload only).
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	return s.rec.Record(ctx, tx, auditSubsystem, action, targetID, after)
}
