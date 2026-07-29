// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the unified cross-type search (review-2026-09 R-26 / D-UnifiedSearch) with
// its D-VisibilityScope trimming (R-30) against a real Postgres — the review's acceptance criteria:
//
//   - a person deep in the directory (page ≥ 2 of the old palette's first-100-per-type fan-out) IS
//     found by a query — the whole point of server-side search;
//
//   - a person outside the subject's read scope is NOT returned even when the trigram matches
//     (visibility folded into the SQL — the PreTrimmed person provider);
//
//   - a provider whose read permission the subject lacks contributes NOTHING (gate, not error);
//
//   - differential parity: the person hits equal ListVisiblePersons for the same subject+query, and
//     the catalog hits equal the module's own list endpoint (R-30 acceptance);
//
//   - the composite keyset token walks multiple providers without skips or duplicates.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/search/...
package search_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzadapters "github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	authzapp "github.com/olegamysk/go-oikumenea/internal/authorization/application"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	"github.com/olegamysk/go-oikumenea/internal/authorization/scope"
	languageadapters "github.com/olegamysk/go-oikumenea/internal/language/adapters"
	languageapp "github.com/olegamysk/go-oikumenea/internal/language/application"
	langdomain "github.com/olegamysk/go-oikumenea/internal/language/domain"
	membershipadapters "github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	membershipapp "github.com/olegamysk/go-oikumenea/internal/membership/application"
	membershipdomain "github.com/olegamysk/go-oikumenea/internal/membership/domain"
	personadapters "github.com/olegamysk/go-oikumenea/internal/person/adapters"
	personapp "github.com/olegamysk/go-oikumenea/internal/person/application"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	searchapp "github.com/olegamysk/go-oikumenea/internal/search/application"
	searchdomain "github.com/olegamysk/go-oikumenea/internal/search/domain"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/bearertoken"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

// world is the full search stack wired the way cmd/oikumenea does it: real person/membership/
// language services, a real PDP-backed enforcer, and the fan-in engine with the same provider
// closures as search_providers.go.
type world struct {
	pool     *pgxpool.Pool
	person   *personapp.Service
	language *languageapp.Service
	engine   *searchapp.Service
}

func newWorld(t *testing.T) world {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
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
	enforcer := pep.New(authzSvc)

	personSvc := personapp.NewService(pool, func(conn pdb.DBTX) persondomain.Repository {
		return personadapters.NewRepository(conn)
	}, audit, func() int { return 720 })
	memSvc := membershipapp.NewService(pool, func(conn pdb.DBTX) membershipdomain.Repository {
		return membershipadapters.NewRepository(conn)
	}, audit)
	personSvc.SetMembershipReader(memSvc)

	languageSvc := languageapp.NewService(pool, func(conn pdb.DBTX) langdomain.Repository {
		return languageadapters.NewRepository(conn)
	})

	engine := searchapp.NewService(
		func(ctx context.Context) (string, bool, error) { return enforcer.SubjectAuthority(ctx) },
		func(ctx context.Context, action string) (bool, error) {
			return enforcer.AllowedAnywhere(ctx, bearertoken.Token(""), action)
		},
	)
	// The same provider shapes cmd/oikumenea/search_providers.go registers.
	mustRegister(t, engine, searchdomain.Provider{
		ObjectType:     "person",
		ReadPermission: string(authzdomain.PermPersonRead),
		PreTrimmed:     true,
		Search: func(ctx context.Context, subject string, isAdmin bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
			var page personapp.Page
			var err error
			if isAdmin {
				page, err = personSvc.ListPersons(ctx, persondomain.PersonFilter{Query: q}, limit, after)
			} else {
				page, err = personSvc.ListVisiblePersons(ctx, subject, persondomain.PersonFilter{Query: q}, limit, after)
			}
			if err != nil {
				return nil, "", err
			}
			hits := make([]searchdomain.RawHit, 0, len(page.Persons))
			for _, p := range page.Persons {
				hits = append(hits, searchdomain.RawHit{ID: p.ID, Label: p.DisplayName})
			}
			return hits, page.NextPageToken, nil
		},
	}, scope.NewPersonScope(memSvc.SubjectReadablePersonsAmong))
	mustRegister(t, engine, searchdomain.Provider{
		ObjectType:     "languoid",
		ReadPermission: string(authzdomain.PermLanguageRead),
		Search: func(ctx context.Context, _ string, _ bool, q, after string, limit int) ([]searchdomain.RawHit, string, error) {
			rows, next, err := languageSvc.ListLanguoidsPage(ctx, langdomain.Filter{Query: q, After: after, Limit: limit})
			if err != nil {
				return nil, "", err
			}
			hits := make([]searchdomain.RawHit, 0, len(rows))
			for _, r := range rows {
				hits = append(hits, searchdomain.RawHit{ID: r.ID, Label: r.Name, Snippet: r.Code})
			}
			return hits, next, nil
		},
	}, scope.NewCatalogScope())

	return world{pool: pool, person: personSvc, language: languageSvc, engine: engine}
}

func mustRegister(t *testing.T, s *searchapp.Service, p searchdomain.Provider, v scope.Visibility) {
	t.Helper()
	if err := s.Register(p, v); err != nil {
		t.Fatalf("register %s: %v", p.ObjectType, err)
	}
}

func subjectCtx(personID string) context.Context {
	return authn.NewContext(context.Background(), authn.Subject{PersonID: personID})
}

const ensureOrgSQL = `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('test-domain','Test Domain')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'test-org','Test Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='test-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`

func (w world) seedUnit(t *testing.T, tag string) string {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), ensureOrgSQL); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	var id string
	if err := w.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (org_id, domain_id, code, name)
		 SELECT o.id, o.domain_id, $1, 'Unit' FROM oikumenea.tenant_organizations o WHERE o.code='test-org'
		 RETURNING id`, "srch-"+tag+"-"+uuid.NewString()[:8]).Scan(&id); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return id
}

func (w world) seedPerson(t *testing.T, displayName, unitID string) string {
	t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ($1) RETURNING id`, displayName).Scan(&id); err != nil {
		t.Fatalf("seed person %q: %v", displayName, err)
	}
	if unitID != "" {
		if _, err := w.pool.Exec(context.Background(),
			`INSERT INTO oikumenea.membership_memberships (person_id, unit_id) VALUES ($1, $2)`, id, unitID); err != nil {
			t.Fatalf("seed membership: %v", err)
		}
	}
	return id
}

// seedGrant gives subject a fresh role carrying the given permission codes, unit-scoped on unitID.
func (w world) seedGrant(t *testing.T, subjectID, unitID string, perms ...string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := w.pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Search test role') RETURNING id`,
		"srch-role-"+uuid.NewString()[:8]).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	for _, p := range perms {
		if _, err := w.pool.Exec(ctx,
			`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, $2)`, roleID, p); err != nil {
			t.Fatalf("seed permission %s: %v", p, err)
		}
	}
	if _, err := w.pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
		 VALUES ($1, $2, $3, 'unit')`, subjectID, roleID, unitID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
}

func (w world) seedLanguoid(t *testing.T, name string) string {
	t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.language_languoids (code, level, name) VALUES ($1, 'language', $2) RETURNING id`,
		uuid.NewString()[:8], name).Scan(&id); err != nil {
		t.Fatalf("seed languoid %q: %v", name, err)
	}
	return id
}

func hitIDs(hits []searchdomain.Hit, objectType string) []string {
	var out []string
	for _, h := range hits {
		if h.ObjectType == objectType {
			out = append(out, h.RID)
		}
	}
	return out
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// TestSearchFindsDeepPersonAndTrims covers three acceptance criteria at once: the deep hit (a
// person past the old palette's first-100 window), the visibility trim (a matching person in an
// unreadable unit is absent), and the person differential (hits ≡ ListVisiblePersons).
func TestSearchFindsDeepPersonAndTrims(t *testing.T) {
	w := newWorld(t)
	tag := strings.ToLower(uuid.NewString()[:8])
	q := "ndl" + tag

	unitA := w.seedUnit(t, "a")
	unitB := w.seedUnit(t, "b")
	// 110 visible filler persons so the target sits beyond any first-100-per-type window.
	for i := 0; i < 110; i++ {
		w.seedPerson(t, fmt.Sprintf("srch bulk %s %03d", tag, i), unitA)
	}
	target := w.seedPerson(t, "Zebra "+q+" Target", unitA)
	hidden := w.seedPerson(t, "Hidden "+q+" Person", unitB) // matches the query, out of reach

	reader := w.seedPerson(t, "srch reader "+tag, "")
	w.seedGrant(t, reader, unitA, "person.read")

	page, err := w.engine.SearchObjects(subjectCtx(reader), q, "", 10, 50, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	persons := hitIDs(page.Hits, "person")
	if !contains(persons, target) {
		t.Fatalf("deep person not found: hits %v", persons)
	}
	if contains(persons, hidden) {
		t.Fatal("visibility trim failed: unreadable person returned")
	}

	// Differential parity (R-30 acceptance): engine person hits ≡ ListVisiblePersons(same subject,
	// same query).
	direct, err := w.person.ListVisiblePersons(subjectCtx(reader), reader, persondomain.PersonFilter{Query: q}, 50, "")
	if err != nil {
		t.Fatalf("ListVisiblePersons: %v", err)
	}
	if len(direct.Persons) != len(persons) {
		t.Fatalf("differential mismatch: engine %d, module %d", len(persons), len(direct.Persons))
	}
	for i, p := range direct.Persons {
		if persons[i] != p.ID {
			t.Fatalf("differential order mismatch at %d: %s vs %s", i, persons[i], p.ID)
		}
	}
}

// TestCatalogPermissionGateAndDifferential: a subject without language.read receives ZERO languoid
// hits for a matching query (the provider is skipped, not errored); a subject holding it receives
// exactly what the language module's own list endpoint returns (catalog differential).
func TestCatalogPermissionGateAndDifferential(t *testing.T) {
	w := newWorld(t)
	tag := strings.ToLower(uuid.NewString()[:8])
	q := "ndl" + tag

	want := map[string]bool{}
	for _, n := range []string{"One", "Two", "Three"} {
		want[w.seedLanguoid(t, "Needlish "+q+" "+n)] = true
	}

	unit := w.seedUnit(t, "lang")
	personOnly := w.seedPerson(t, "srch personly "+tag, "")
	w.seedGrant(t, personOnly, unit, "person.read")
	langReader := w.seedPerson(t, "srch langreader "+tag, "")
	w.seedGrant(t, langReader, unit, "language.read")

	page, err := w.engine.SearchObjects(subjectCtx(personOnly), q, "", 10, 50, "")
	if err != nil {
		t.Fatalf("search without language.read: %v", err)
	}
	if got := hitIDs(page.Hits, "languoid"); len(got) != 0 {
		t.Fatalf("permission gate failed: languoid hits for a subject without language.read: %v", got)
	}

	page, err = w.engine.SearchObjects(subjectCtx(langReader), q, "", 10, 50, "")
	if err != nil {
		t.Fatalf("search with language.read: %v", err)
	}
	got := hitIDs(page.Hits, "languoid")
	if len(got) != len(want) {
		t.Fatalf("languoid hits %v, want the %d seeded", got, len(want))
	}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("unexpected languoid hit %s", id)
		}
	}

	// Catalog differential: engine hits ≡ the module's own filtered list.
	rows, _, err := w.language.ListLanguoidsPage(context.Background(), langdomain.Filter{Query: q, Limit: 50})
	if err != nil {
		t.Fatalf("ListLanguoidsPage: %v", err)
	}
	if len(rows) != len(got) {
		t.Fatalf("catalog differential mismatch: engine %d, module %d", len(got), len(rows))
	}
}

// TestTokenWalkAcrossProviders drains a query matching both providers with perTypeLimit=1: the
// composite token must walk every hit exactly once and terminate.
func TestTokenWalkAcrossProviders(t *testing.T) {
	w := newWorld(t)
	tag := strings.ToLower(uuid.NewString()[:8])
	q := "ndl" + tag

	unit := w.seedUnit(t, "walk")
	want := map[string]bool{}
	for _, n := range []string{"Alpha", "Beta", "Gamma"} {
		want[w.seedPerson(t, "Walk "+q+" "+n, unit)] = true
		want[w.seedLanguoid(t, "Walkish "+q+" "+n)] = true
	}
	reader := w.seedPerson(t, "srch walker "+tag, "")
	w.seedGrant(t, reader, unit, "person.read", "language.read")

	seen := map[string]int{}
	token := ""
	for i := 0; i < 20; i++ {
		page, err := w.engine.SearchObjects(subjectCtx(reader), q, "", 1, 50, token)
		if err != nil {
			t.Fatalf("walk page %d: %v", i, err)
		}
		for _, h := range page.Hits {
			seen[h.RID]++
		}
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	if len(seen) != len(want) {
		t.Fatalf("walk saw %d ids, want %d", len(seen), len(want))
	}
	for id := range want {
		if seen[id] != 1 {
			t.Fatalf("id %s seen %d times", id, seen[id])
		}
	}
}
