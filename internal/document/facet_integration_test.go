// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the top-level document listing and its facet filters (M56 ticket 3 /
// D-ObjectFacets) against a real Postgres.
//
// Documents differ from memberships and orders in HOW they are scoped: document_documents carries no
// unit column and no RLS policy, so visibility runs THROUGH THE HOLDER (D-PersonReadScope) — the
// scoped query folds a semi-join on the holder's active memberships against the subject's reach.
// That makes the intersection assertion here holder-keyed rather than unit-keyed, and it is the one
// place a "documents of people I cannot see" leak would show up.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/document/... -run Facet
package document_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/document/application"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
)

func dsp(s string) *string    { return &s }
func ddp(s string) *time.Time { t, _ := time.Parse(domain.ISODate, s); return &t }

// dFacetWorld: two holders, one inside the reader's reach and one outside, each with documents
// covering every facet. The out-of-reach holder is what makes the scope assertion meaningful — a
// query that forgot the holder semi-join would return their documents too.
type dFacetWorld struct {
	svc  *application.Service
	pool *pgxpool.Pool

	reader          string
	unitIn, unitOut string
	holderIn        string
	holderOut       string

	typeA, typeB string
	country      string

	dActive     string // holderIn, typeA, active,     issued 2024-05-06, expires 2030-01-01, country set
	dSuperseded string // holderIn, typeB, superseded, issued 2020-02-03, NO expiry, no country
	dOut        string // holderOut, typeA, active,     issued 2024-05-06
}

func seedDFacetWorld(t *testing.T) *dFacetWorld {
	t.Helper()
	svc, pool := newService(t)
	w := &dFacetWorld{svc: svc, pool: pool}

	w.unitIn = seedFacetUnit(t, pool)
	w.unitOut = seedFacetUnit(t, pool)
	w.country = countryRID(t, pool, "UA")

	w.typeA = seedFacetDocType(t, pool)
	w.typeB = seedFacetDocType(t, pool)

	w.holderIn = seedPerson(t, pool)
	w.holderOut = seedPerson(t, pool)
	seedFacetMembership(t, pool, w.holderIn, w.unitIn)
	seedFacetMembership(t, pool, w.holderOut, w.unitOut)

	w.dActive = seedFacetDocument(t, pool, w.holderIn, w.typeA, "active", "2024-05-06", "2030-01-01", w.country)
	w.dSuperseded = seedFacetDocument(t, pool, w.holderIn, w.typeB, "superseded", "2020-02-03", "", "")
	w.dOut = seedFacetDocument(t, pool, w.holderOut, w.typeA, "active", "2024-05-06", "2030-01-01", w.country)

	w.reader = seedPerson(t, pool)
	seedFacetMembership(t, pool, w.reader, w.unitIn)
	seedDocumentReadGrant(t, pool, w.reader, w.unitIn)
	return w
}

// dFacetEnsureOrgSQL idempotently seeds a test domain + organization + its per-org authority-bearing
// graphs (D-TenantOrganizations, M40). A unit needs a real org: the M55 authz_unit_org projection
// trigger derives org_id from it and rejects a NULL.
const dFacetEnsureOrgSQL = `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('test-domain','Test Domain')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'test-org','Test Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='test-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_graphs (org_id, code, name, is_default, is_authority_bearing)
  SELECT o.id, 'command', 'Command', true, true FROM oikumenea.tenant_organizations o WHERE o.code='test-org'
  ON CONFLICT (org_id, code) WHERE deleted_at IS NULL AND org_id IS NOT NULL DO NOTHING`

func seedFacetUnit(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	if _, err := pool.Exec(context.Background(), dFacetEnsureOrgSQL); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (org_id, domain_id, code, name)
		 SELECT o.id, o.domain_id, $1, 'Facet unit' FROM oikumenea.tenant_organizations o WHERE o.code='test-org'
		 RETURNING id`,
		code(t, "dfacet-unit")).Scan(&id); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return id
}

func seedFacetMembership(t *testing.T, pool *pgxpool.Pool, personID, unitID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO oikumenea.membership_memberships (person_id, unit_id, status) VALUES ($1, $2, 'active')`,
		personID, unitID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func seedFacetDocType(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.document_document_types (code, name) VALUES ($1, '{"eng":"Facet type"}'::jsonb) RETURNING id`,
		code(t, "dfacet-type")).Scan(&id); err != nil {
		t.Fatalf("seed document type: %v", err)
	}
	return id
}

func seedFacetDocument(t *testing.T, pool *pgxpool.Pool, personID, typeID, status, issuedOn, expiresOn, countryID string) string {
	t.Helper()
	var issued, expires, country any
	if issuedOn != "" {
		issued = issuedOn
	}
	if expiresOn != "" {
		expires = expiresOn
	}
	if countryID != "" {
		country = countryID
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.document_documents (person_id, type_id, status, issued_on, expires_on, issuing_country_id)
		 VALUES ($1, $2, $3, $4::date, $5::date, $6) RETURNING id`,
		personID, typeID, status, issued, expires, country).Scan(&id); err != nil {
		t.Fatalf("seed document: %v", err)
	}
	return id
}

func seedDocumentReadGrant(t *testing.T, pool *pgxpool.Pool, readerID, unitID string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Document facet test role') RETURNING id`,
		code(t, "dfacet-role")).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, 'document.read')`,
		roleID); err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
		 VALUES ($1, $2, $3, 'unit')`, readerID, roleID, unitID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
}

func dAllIDs(t *testing.T, list func(pageToken string) (application.DocumentPage, error)) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	token := ""
	for i := 0; ; i++ {
		if i > 2000 {
			t.Fatal("paging did not terminate after 2000 pages")
		}
		page, err := list(token)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		for _, d := range page.Documents {
			out[d.ID] = true
		}
		if page.NextPageToken == "" {
			return out
		}
		token = page.NextPageToken
	}
}

func (w *dFacetWorld) admin(t *testing.T, f domain.DocumentFilter) map[string]bool {
	t.Helper()
	return dAllIDs(t, func(tok string) (application.DocumentPage, error) {
		return w.svc.ListDocuments(context.Background(), f, 50, tok)
	})
}

func (w *dFacetWorld) scoped(t *testing.T, f domain.DocumentFilter) map[string]bool {
	t.Helper()
	return dAllIDs(t, func(tok string) (application.DocumentPage, error) {
		return w.svc.ListVisibleDocuments(context.Background(), w.reader, f, 50, tok)
	})
}

func TestDocumentFacetsRoundTrip(t *testing.T) {
	w := seedDFacetWorld(t)

	cases := []struct {
		name              string
		filter            domain.DocumentFilter
		wantIn, wantOutOf []string
	}{
		{
			"typeId", domain.DocumentFilter{TypeID: dsp(w.typeA)},
			[]string{w.dActive, w.dOut}, []string{w.dSuperseded},
		},
		{
			"status=superseded", domain.DocumentFilter{Status: dsp("superseded")},
			[]string{w.dSuperseded}, []string{w.dActive, w.dOut},
		},
		{
			"issuingCountryId", domain.DocumentFilter{IssuingCountryID: dsp(w.country)},
			[]string{w.dActive}, []string{w.dSuperseded},
		},
		{
			"issuedOn range", domain.DocumentFilter{
				IssuedOnFrom: ddp("2024-01-01"), IssuedOnTo: ddp("2024-12-31"),
			},
			[]string{w.dActive, w.dOut}, []string{w.dSuperseded},
		},
		{
			// A document with NO expiry must be excluded by any expiry bound — the (no expiry) set is
			// a distinct bucket on M57's stats endpoint, not a filterable value.
			"expiresOn range excludes no-expiry", domain.DocumentFilter{
				ExpiresOnFrom: ddp("2029-01-01"), ExpiresOnTo: ddp("2031-01-01"),
			},
			[]string{w.dActive}, []string{w.dSuperseded},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := w.admin(t, c.filter)
			for _, id := range c.wantIn {
				if !got[id] {
					t.Errorf("admin: %s missing from the filtered set", id)
				}
			}
			for _, id := range c.wantOutOf {
				if got[id] {
					t.Errorf("admin: %s should not match this filter", id)
				}
			}
			assertDocScopedIsAdminIntersectReach(t, w, c.filter)
		})
	}
}

// TestDocumentScopedHidesUnreachableHolders is the leak test: the out-of-reach holder's documents
// must never appear on the scoped path, under ANY filter — including the filters that match them.
func TestDocumentScopedHidesUnreachableHolders(t *testing.T) {
	w := seedDFacetWorld(t)
	for _, f := range []domain.DocumentFilter{
		{},
		{TypeID: dsp(w.typeA)},
		{Status: dsp("active")},
		{IssuingCountryID: dsp(w.country)},
		{IssuedOnFrom: ddp("2024-01-01")},
	} {
		scoped := w.scoped(t, f)
		if scoped[w.dOut] {
			t.Errorf("scoped listing leaked %s — a document of a holder outside the reader's reach", w.dOut)
		}
		if !scoped[w.dActive] {
			t.Errorf("scoped listing omitted %s, whose holder IS in the reader's reach", w.dActive)
		}
	}
}

func TestDocumentScopedEqualsAdminIntersectReach(t *testing.T) {
	w := seedDFacetWorld(t)
	for _, f := range []domain.DocumentFilter{
		{},
		{TypeID: dsp(w.typeA)},
		{TypeID: dsp(w.typeB)},
		{Status: dsp("active")},
		{Status: dsp("superseded")},
		{IssuingCountryID: dsp(w.country)},
	} {
		assertDocScopedIsAdminIntersectReach(t, w, f)
	}
}

func assertDocScopedIsAdminIntersectReach(t *testing.T, w *dFacetWorld, f domain.DocumentFilter) {
	t.Helper()
	admin := w.admin(t, f)
	scoped := w.scoped(t, f)

	// Expected: the admin set restricted to documents whose holder has an active membership in the
	// reader's one readable unit. Computed from the DB so the other suites' rows in this shared
	// database cannot make the assertion vacuous.
	readable := map[string]bool{}
	rows, err := w.pool.Query(context.Background(),
		`SELECT d.id FROM oikumenea.document_documents d
		 WHERE d.deleted_at IS NULL
		   AND EXISTS (SELECT 1 FROM oikumenea.membership_memberships m
		               WHERE m.person_id = d.person_id AND m.status='active' AND m.deleted_at IS NULL
		                 AND m.unit_id = $1)`, w.unitIn)
	if err != nil {
		t.Fatalf("read holder-visible documents: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		readable[id] = true
	}

	for id := range scoped {
		if !admin[id] {
			t.Errorf("scoped returned %s, which admin with the same filter does not — the scoped path "+
				"must never be WIDER than admin", id)
		}
		if !readable[id] {
			t.Errorf("scoped returned %s, whose holder is outside the reader's reach", id)
		}
	}
	for id := range admin {
		if readable[id] && !scoped[id] {
			t.Errorf("scoped omitted %s, which is both admin-visible and holder-readable — the two "+
				"filter blocks have drifted", id)
		}
	}
}

// TestDocumentFilteredPagingDrainsExactly is the R-06 assertion. It matters most on the scoped path:
// the holder semi-join is the predicate most tempting to apply in Go after the page is cut, which
// would hand back a short page WITH a nextPageToken.
func TestDocumentFilteredPagingDrainsExactly(t *testing.T) {
	w := seedDFacetWorld(t)
	f := domain.DocumentFilter{TypeID: dsp(w.typeA)}

	bulk := w.admin(t, f)
	drain := dAllIDs(t, func(tok string) (application.DocumentPage, error) {
		return w.svc.ListDocuments(context.Background(), f, 1, tok)
	})
	if len(bulk) != len(drain) {
		t.Fatalf("pageSize=1 drain yielded %d rows, bulk yielded %d", len(drain), len(bulk))
	}

	scopedBulk := w.scoped(t, f)
	scopedDrain := dAllIDs(t, func(tok string) (application.DocumentPage, error) {
		return w.svc.ListVisibleDocuments(context.Background(), w.reader, f, 1, tok)
	})
	if len(scopedBulk) != len(scopedDrain) {
		t.Fatalf("scoped pageSize=1 drain yielded %d rows, bulk yielded %d", len(scopedDrain), len(scopedBulk))
	}
	for id := range scopedBulk {
		if !scopedDrain[id] {
			t.Errorf("%s appears in the scoped bulk page but not in the one-at-a-time drain", id)
		}
	}
}

func TestDocumentFilterValidation(t *testing.T) {
	w := seedDFacetWorld(t)
	ctx := context.Background()

	for _, c := range []struct {
		name string
		f    domain.DocumentFilter
	}{
		{"unknown status", domain.DocumentFilter{Status: dsp("expired")}},
		{"non-RID typeId", domain.DocumentFilter{TypeID: dsp("not-a-rid")}},
		{"inverted issuedOn", domain.DocumentFilter{
			IssuedOnFrom: ddp("2025-01-01"), IssuedOnTo: ddp("2024-01-01"),
		}},
		{"inverted expiresOn", domain.DocumentFilter{
			ExpiresOnFrom: ddp("2031-01-01"), ExpiresOnTo: ddp("2030-01-01"),
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := w.svc.ListDocuments(ctx, c.f, 50, ""); err == nil {
				t.Error("admin path accepted an invalid filter")
			}
			if _, err := w.svc.ListVisibleDocuments(ctx, w.reader, c.f, 50, ""); err == nil {
				t.Error("scoped path accepted an invalid filter")
			}
		})
	}
}
