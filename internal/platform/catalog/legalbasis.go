// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package catalog holds the platform module's cross-cutting reference catalogs (D-OverlayFoundation,
// M29). Today: the GDPR lawful-basis catalog (platform_legal_basis_kinds), referenced by FK from every
// future pii:special overlay store (M31+). It is a thin raw-pgx repository + an audited write path —
// it has no hexagonal sub-layering of its own because the platform module owns infrastructure, not a
// domain aggregate (overview.md).
package catalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// ErrInvalid is returned for a malformed legal-basis upsert (bad code/name/article).
var ErrInvalid = errors.New("invalid legal-basis request")

// LegalBasisKind is one GDPR lawful basis (Article 6) or special-category condition (Article 9).
type LegalBasisKind struct {
	Code      string
	Name      string
	Article   string // art6 | art9
	Status    string // active | retired
	SortOrder *int
}

// Service reads + (instance-admin) upserts the lawful-basis catalog, auditing writes (D-Audit).
type Service struct {
	pool  *pgxpool.Pool
	audit *auditapp.Service
}

// NewService builds the catalog service over the platform pool + the audit service.
func NewService(pool *pgxpool.Pool, audit *auditapp.Service) *Service {
	return &Service{pool: pool, audit: audit}
}

// List returns the lawful-basis catalog ordered by (article, sort_order, code).
func (s *Service) List(ctx context.Context) ([]LegalBasisKind, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT code, name, article, status, sort_order
		FROM oikumenea.platform_legal_basis_kinds
		WHERE deleted_at IS NULL
		ORDER BY article, sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LegalBasisKind, 0, 16)
	for rows.Next() {
		var k LegalBasisKind
		if err := rows.Scan(&k.Code, &k.Name, &k.Article, &k.Status, &k.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// Upsert adds or updates a lawful-basis catalog entry (instance-admin), recording an audit row in the
// same transaction (D-Audit). article must be art6 | art9.
func (s *Service) Upsert(ctx context.Context, k LegalBasisKind) (LegalBasisKind, error) {
	if k.Code == "" || k.Name == "" || (k.Article != "art6" && k.Article != "art9") {
		return LegalBasisKind{}, ErrInvalid
	}
	var out LegalBasisKind
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO oikumenea.platform_legal_basis_kinds (code, name, article, sort_order)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name, article = EXCLUDED.article, sort_order = EXCLUDED.sort_order,
				status = 'active', deleted_at = NULL
			RETURNING code, name, article, status, sort_order`,
			k.Code, k.Name, k.Article, k.SortOrder)
		if err := row.Scan(&out.Code, &out.Name, &out.Article, &out.Status, &out.SortOrder); err != nil {
			return err
		}
		return s.record(ctx, tx, "legal-basis.upsert", out.Code, map[string]any{"code": out.Code, "article": out.Article})
	})
	return out, err
}

// inTx runs fn in a transaction, committing on success and rolling back on error.
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

// record mints a platform Action RID (service=1, kind=action=3, generic type=0) and writes the audit
// row in the caller's transaction (D-Audit), as a `system` actor under the catalog subsystem.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetID string, after any) error {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(1, 3, 0)").Scan(&rid); err != nil {
		return err
	}
	raw, _ := json.Marshal(after)
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  "platform-catalog",
		Action:     action,
		TargetType: "legal_basis_kind",
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      raw,
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}
