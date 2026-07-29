// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Scale-world measurement harness (review-2026-07 Phase 0 / M46). Runs the authorization request
// path against the synthetic national-scale world seeded by scripts/seed-scale and logs the numbers
// the review's Phase-1 acceptance criteria compare against (R-01 query counts, R-02 reach/GUC size,
// R-03 round trips). It asserts nothing about performance — it measures; the "before"/"after"
// numbers land in docs/architecture/review-2026-07.md § Measurements.
//
//	OIKUMENEA_SCALE_DSN="postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable" \
//	  go test -tags integration -run TestScaleMeasure -v ./internal/authorization/
package authorization_test

import (
	"context"
	"os"
	"testing"
	"time"

	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzadapters "github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	authzapp "github.com/olegamysk/go-oikumenea/internal/authorization/application"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	membershipadapters "github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

func TestScaleMeasure(t *testing.T) {
	dsn := os.Getenv("OIKUMENEA_SCALE_DSN")
	if dsn == "" {
		t.Skip("set OIKUMENEA_SCALE_DSN to the seed-scale database (scripts/seed-scale) to run the measurement harness")
	}
	ctx := context.Background()
	pool, err := pdb.NewPool(ctx, dsn, "local")
	if err != nil {
		t.Fatalf("connect scale db: %v", err)
	}
	t.Cleanup(pool.Close)

	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	tenantSvc := tenantapp.NewService(pool, func(conn pdb.DBTX) tenantdomain.Repository {
		return tenantadapters.NewRepository(conn)
	}, audit)
	authzSvc := authzapp.NewService(pool, func(conn pdb.DBTX) authzdomain.Repository {
		return authzadapters.NewRepository(conn)
	}, audit, authzdomain.NewPDP(tenantSvc), tenantSvc, func(conn pdb.DBTX) authzdomain.PrincipalRepository {
		return authzadapters.NewRepository(conn)
	})
	memRepo := membershipadapters.NewRepository(pool)

	var anyUnit string
	if err := pool.QueryRow(ctx, "SELECT id FROM oikumenea.tenant_units LIMIT 1").Scan(&anyUnit); err != nil {
		t.Fatalf("pick a unit: %v", err)
	}

	for _, code := range []string{"scale-root-subject", "scale-mid-subject", "scale-leaf-subject"} {
		code := code
		t.Run(code, func(t *testing.T) {
			var subject string
			if err := pool.QueryRow(ctx,
				"SELECT id FROM oikumenea.person_persons WHERE code = $1", code).Scan(&subject); err != nil {
				t.Fatalf("probe subject %s not found (run scripts/seed-scale first): %v", code, err)
			}

			// (1) + (2) Authority resolution + RLS GUC state: since R-01/R-02 this is one cached
			// authority fetch and an O(1) GUC payload (no reach materialization exists to time).
			start := time.Now()
			_, state, err := authzSvc.ContextWithAuthority(ctx, subject)
			if err != nil {
				t.Fatalf("ContextWithAuthority: %v", err)
			}
			gucBytes := len(state.PersonID) + len("true")
			t.Logf("ContextWithAuthority: %v  GUC payload=%d bytes", time.Since(start).Round(time.Millisecond), gucBytes)

			// (3) Connection pinning: acquire + GUC set/reset round trips (R-03).
			cctx, counter := pdb.WithQueryCounter(ctx)
			counter.CaptureSQL()
			start = time.Now()
			_, release, err := pdb.AcquireScoped(cctx, pool, state)
			if err != nil {
				t.Fatalf("AcquireScoped: %v", err)
			}
			acquire := time.Since(start)
			release()
			t.Logf("AcquireScoped+release: %v  set_config statements=%d", acquire.Round(time.Millisecond), counter.CountContaining("set_config"))

			// (4) Simulated guarded request: authenticator authority fetch + a 3-gate handler (R-01).
			// The handler gates consume the snapshot the returned context carries; within the grant
			// cache TTL even the authenticator fetch costs zero authority queries.
			cctx, counter = pdb.WithQueryCounter(ctx)
			counter.CaptureSQL()
			start = time.Now()
			actx, _, err := authzSvc.ContextWithAuthority(cctx, subject)
			if err != nil {
				t.Fatalf("ContextWithAuthority: %v", err)
			}
			for i := 0; i < 3; i++ {
				_ = authzSvc.Enforce(actx, subject, "person.read", anyUnit) // denial is fine; we count queries
			}
			t.Logf("guarded request (ContextWithAuthority + 3×Enforce): %v  total queries=%d  grants joins=%d  admin checks=%d",
				time.Since(start).Round(time.Millisecond), counter.Count(),
				counter.CountContaining("authz_role_assignments"), counter.CountContaining("authz_instance_admins"))

			// (5) Visible-persons page: the R-02.1 semi-join (the reach set never leaves Postgres).
			start = time.Now()
			ids, err := memRepo.VisiblePersonIDsForSubject(ctx, subject, "", persondomain.PersonFilter{}, 51)
			if err != nil {
				t.Fatalf("VisiblePersonIDsForSubject: %v", err)
			}
			t.Logf("visible-persons page (semi-join, LIMIT 51): %v  rows=%d", time.Since(start).Round(time.Millisecond), len(ids))
		})
	}
}
