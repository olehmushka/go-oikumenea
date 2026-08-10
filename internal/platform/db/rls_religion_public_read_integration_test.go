// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for the religion discovery PUBLIC-read RLS bypass (GH-34, migration 0025). Companion
// to rls_service_arm_integration_test.go: it proves the fix is exactly as narrow as intended —
//   - a service principal holding ONLY an instance-wide `religion.read` grant (org_id NULL) now sees a
//     `public` religion_sites row (the discovery repro from GH-34);
//   - the SAME principal still cannot see a `private` religion_sites row under the same unit — the
//     public-read bypass does not widen non-public reach;
//   - a principal holding an org-CONFINED grant for a DIFFERENT org still sees neither row — the
//     M55 "instance-wide grant confers no operational reach" boundary (authz_principal_org_in_reach)
//     is untouched by this migration;
//   - a principal holding an org-confined grant for the SAME org sees both rows (the pre-existing
//     reach-based policy, unchanged).
//
// Run (same DSN contract as rls_service_arm_integration_test.go):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestRLSReligionPublicRead ./internal/platform/db/...
package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// religionSiteFixture: one org + one org unit, a PUBLIC and a PRIVATE religion_sites row under that
// unit (sharing one location + site type), and a machine principal whose grants each subtest sets.
type religionSiteFixture struct {
	orgO        string // organization RID
	unit        string // org unit RID
	publicSite  string // religion_sites RID, visibility='public'
	privateSite string // religion_sites RID, visibility='private'
	principal   string // service-principal RID
}

func seedReligionSiteFixture(t *testing.T, super *pgxpool.Pool) religionSiteFixture {
	t.Helper()
	ctx := context.Background()
	if _, err := super.Exec(ctx, `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('rls-religion-domain','RLS Religion Domain')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'rls-religion-org','RLS Religion Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='rls-religion-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("seed religion org: %v", err)
	}

	var f religionSiteFixture
	var orgDomain string
	if err := super.QueryRow(ctx,
		`SELECT id, domain_id FROM oikumenea.tenant_organizations WHERE code='rls-religion-org'`).Scan(&f.orgO, &orgDomain); err != nil {
		t.Fatalf("read org: %v", err)
	}

	if err := super.QueryRow(ctx, `
		INSERT INTO oikumenea.tenant_units (name, visibility, org_id, domain_id)
		VALUES ('RLS religion unit', 'shadow', $1, $2) RETURNING id`, f.orgO, orgDomain).Scan(&f.unit); err != nil {
		t.Fatalf("seed unit: %v", err)
	}

	var loc string
	if err := super.QueryRow(ctx, `
		INSERT INTO oikumenea.location_locations (geom, country_id)
		VALUES (ST_SetSRID(ST_MakePoint(30.5234,50.4501),4326)::geography,
		        (SELECT id FROM oikumenea.geo_countries WHERE code='UA'))
		RETURNING id`).Scan(&loc); err != nil {
		t.Fatalf("seed location: %v", err)
	}

	var siteType string
	if err := super.QueryRow(ctx,
		`SELECT id FROM oikumenea.religion_site_types WHERE code='church' AND deleted_at IS NULL ORDER BY id LIMIT 1`).Scan(&siteType); err != nil {
		t.Fatalf("resolve site type: %v", err)
	}

	site := func(visibility string) string {
		var id string
		if err := super.QueryRow(ctx, `
			INSERT INTO oikumenea.religion_sites (org_unit_id, location_id, site_type_id, visibility)
			VALUES ($1,$2,$3,$4) RETURNING id`, f.unit, loc, siteType, visibility).Scan(&id); err != nil {
			t.Fatalf("seed %s site: %v", visibility, err)
		}
		return id
	}
	f.publicSite = site("public")
	f.privateSite = site("private")

	if err := super.QueryRow(ctx, `
		INSERT INTO oikumenea.account_service_principals (code, name, issuer, subject, status)
		VALUES ('rls-religion-connector','RLS Religion Connector','urn:test:rls-religion','rls-religion-subject','active')
		ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&f.principal); err != nil {
		t.Fatalf("seed principal: %v", err)
	}
	return f
}

// religionSiteVisible reports whether the given religion_sites id is selectable on the querier.
func religionSiteVisible(ctx context.Context, t *testing.T, q rowQuerier, id string) bool {
	t.Helper()
	var got string
	err := q.QueryRow(ctx, "SELECT id FROM oikumenea.religion_sites WHERE id = $1", id).Scan(&got)
	if err == nil {
		return true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	t.Fatalf("query religion_sites %s: %v", id, err)
	return false
}

func TestRLSReligionPublicRead(t *testing.T) {
	ctx := context.Background()
	super, err := pdb.NewPool(ctx, superuserDSN(), "local")
	if err != nil {
		t.Skipf("no test database (set OIKUMENEA_TEST_DSN): %v", err)
	}
	defer super.Close()
	f := seedReligionSiteFixture(t, super)

	app, err := pdb.NewPool(ctx, restrictedDSN(t), "local")
	if err != nil {
		t.Skipf("restricted role not provisioned: %v", err)
	}
	defer app.Close()

	orgP := "" // a different, unrelated org for the negative case below
	if err := super.QueryRow(ctx, `
		INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
		SELECT 'rls-religion-org-p','RLS Religion Org P', d.id FROM oikumenea.tenant_domains d WHERE d.code='rls-religion-domain'
		ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&orgP); err != nil {
		t.Fatalf("seed org P: %v", err)
	}

	t.Run("instance-wide-only grant sees the public site but not the private one", func(t *testing.T) {
		setGrants(t, super, f.principal, [2]string{"religion.read", ""})
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !religionSiteVisible(ctx, t, conn, f.publicSite) {
			t.Error("a public religion_sites row must be visible to an instance-wide religion.read grant (GH-34)")
		}
		if religionSiteVisible(ctx, t, conn, f.privateSite) {
			t.Error("a private religion_sites row must stay hidden from an instance-wide grant")
		}
	})

	t.Run("org-confined grant for a DIFFERENT org still sees the public site (unconditional bypass) but not the private one", func(t *testing.T) {
		// religion_sites_public_read is a SECOND, unconditional permissive policy — Postgres OR-combines
		// it with religion_sites_reach, exactly like tenant_units_public_read. It does not check grant
		// identity at all (that's the PEP's job); it only mirrors the app layer's "public is public"
		// decision. The boundary this preserves is that the PRIVATE row stays reach-gated regardless.
		setGrants(t, super, f.principal, [2]string{"religion.read", orgP})
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !religionSiteVisible(ctx, t, conn, f.publicSite) {
			t.Error("a public religion_sites row must be visible regardless of which org the caller's grant confines it to")
		}
		if religionSiteVisible(ctx, t, conn, f.privateSite) {
			t.Error("a private religion_sites row must stay hidden from a grant confined to an unrelated org")
		}
	})

	t.Run("org-confined grant for the SAME org sees both rows (pre-existing reach)", func(t *testing.T) {
		setGrants(t, super, f.principal, [2]string{"religion.read", f.orgO})
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !religionSiteVisible(ctx, t, conn, f.publicSite) {
			t.Error("an org-confined grant for the site's own org must see the public site")
		}
		if !religionSiteVisible(ctx, t, conn, f.privateSite) {
			t.Error("an org-confined grant for the site's own org must see the private site too")
		}
	})
}
