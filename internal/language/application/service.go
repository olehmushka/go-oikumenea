// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application holds the language module's application service — the read-only orchestrator the
// transport layer calls to browse the Glottolog languoid forest + ISO-15924 writing systems, and that
// other modules could call in-process to resolve a language. It depends on the domain port and the
// platform DB surface; it never imports the adapters package directly (the repository factory is
// injected by module.go).
package application

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/language/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// defaultLimit / maxLimit bound a languoid listing (the catalog is ~26k rows; callers narrow via the
// level/family/query filters).
const (
	defaultLimit = 100
	maxLimit     = 1000
)

// RepositoryFactory binds a domain.Repository to a command surface (the pool for reads). Injected by
// module.go so the application layer never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the language application service. Reads run on the pool directly; it owns no writes.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	labeler stats.Labeler
}

// NewService wires the service with the pool and the repository factory.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory) *Service {
	return &Service{pool: pool, newRepo: newRepo}
}

// SetBucketLabeler injects the dashboard's ref-bucket name resolver (M58 ticket 4 / D-ObjectFacets),
// wired at the composition root.
//
// It is INERT for languoid as declared: the type has no `ref` facet — a glottocode and a macroarea are
// their own labels, and `level`/`status` are enums — so the kernel never calls it. It is wired anyway
// so that adding a ref facet later cannot silently ship a chart whose axis is RID tails.
func (s *Service) SetBucketLabeler(l stats.Labeler) { s.labeler = l }

// LanguoidStats is the languoid dashboard (M58 ticket 4 / D-ObjectFacets): every selected facet's
// distribution plus the total, over EXACTLY the set ListLanguoidsPage would page under the same
// structural filters.
//
// isAdmin=true with an empty subject is the ABSENCE of a visibility decision, not an escalation: the
// languoid registry has no row-level security, no unit column and no reach predicate, so there is
// nothing for a scoped arm to narrow. That is vehicle's and external_organization's shape, and it is
// deliberately not the audit ledger's — where the one arm exists because the RLS policy on the pinned
// connection IS the decision.
func (s *Service) LanguoidStats(ctx context.Context, f domain.Filter, sel stats.Selection) (stats.Result, error) {
	if err := f.Validate(); err != nil {
		return stats.Result{}, err
	}
	return stats.Compute(ctx, s.labeler, sel, true, "", func(string) ([]stats.Group, error) {
		return s.newRepo(s.pool).LanguoidStats(ctx, f, sel)
	})
}

// clampLimit bounds a requested page size to [1, maxLimit], applying the default when unset.
func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

// ListLanguoids returns matching languoids in code order, clamping the limit to [1, maxLimit].
func (s *Service) ListLanguoids(ctx context.Context, f domain.Filter) ([]domain.Languoid, error) {
	f.Limit = clampLimit(f.Limit)
	return s.newRepo(s.pool).ListLanguoids(ctx, f)
}

// ListLanguoidsPage is the paged read for the transport layer: it returns a page of at most the clamped
// limit plus a keyset cursor for the next page (the last glottocode), empty when the page is the last.
// It over-fetches one row past the limit to detect whether more rows exist without a second COUNT query.
func (s *Service) ListLanguoidsPage(ctx context.Context, f domain.Filter) ([]domain.Languoid, string, error) {
	lim := clampLimit(f.Limit)
	f.Limit = lim + 1
	rows, err := s.newRepo(s.pool).ListLanguoids(ctx, f)
	if err != nil {
		return nil, "", err
	}
	if len(rows) > lim {
		rows = rows[:lim]
		return rows, rows[len(rows)-1].Code, nil
	}
	return rows, "", nil
}

// GetLanguoid returns one languoid by RID (found=false when absent).
func (s *Service) GetLanguoid(ctx context.Context, id string) (domain.Languoid, bool, error) {
	return s.newRepo(s.pool).GetLanguoid(ctx, id)
}

// ListWritingSystems returns the ISO-15924 writing systems in code order.
func (s *Service) ListWritingSystems(ctx context.Context) ([]domain.WritingSystem, error) {
	return s.newRepo(s.pool).ListWritingSystems(ctx)
}

// ResolveLanguoids maps glottocodes to languoid RIDs (the M53 wiring resolve seam, D-ConnectorPlane).
// Only found codes appear. Read-only over the pool.
func (s *Service) ResolveLanguoids(ctx context.Context, codes []string) (map[string]string, error) {
	return resolveByCode(ctx, s.pool, "oikumenea.language_languoids", codes)
}

// ResolveWritingSystems maps ISO-15924 codes to writing-system RIDs (M53 wiring resolve seam).
func (s *Service) ResolveWritingSystems(ctx context.Context, codes []string) (map[string]string, error) {
	return resolveByCode(ctx, s.pool, "oikumenea.writing_systems", codes)
}

// resolveByCode returns code→RID for the rows of `table` whose `code` is in `codes`. `table` is a
// package-internal constant (never user input), so interpolating it into the query is safe.
func resolveByCode(ctx context.Context, pool *pgxpool.Pool, table string, codes []string) (map[string]string, error) {
	rows, err := pool.Query(ctx, `SELECT code, id FROM `+table+` WHERE code = ANY($1)`, codes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string, len(codes))
	for rows.Next() {
		var code, id string
		if err := rows.Scan(&code, &id); err != nil {
			return nil, err
		}
		// language_languoids.code is char(8) (glottocodes), so scanned values are space-padded; trim
		// so the map key equals the requested code.
		out[strings.TrimRight(code, " ")] = id
	}
	return out, rows.Err()
}
