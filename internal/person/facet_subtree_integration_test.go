// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the SUBTREE-EXPANDING unitId facet and its `graph` narrowing arg
// (M56 / D-ObjectFacets). This is the one facet whose semantics are not obvious from its name, so
// it gets its own file:
//
//   - filtering by a unit matches people in that unit OR in any closure descendant of it;
//
//   - the expansion spans every AUTHORITY-BEARING graph by default — the same closure set the
//     read-scope predicate itself walks, which is why the filter can never widen what a caller sees;
//
//   - `graph` narrows the expansion to one graph, so a person reachable only through a different
//     graph drops out.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/person/... -run Subtree
package person_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olehmushka/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/olehmushka/go-oikumenea/internal/person/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olehmushka/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olehmushka/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olehmushka/go-oikumenea/internal/tenant/domain"
)

// newTenantService builds a tenant application service over the same pool, so the test can create
// real edges through AddEdge — which is what maintains the transitive-closure table the facet reads.
// Seeding closure rows by hand would test the query against a fiction.
func newTenantService(t *testing.T, pool *pgxpool.Pool) *tenantapp.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	return tenantapp.NewService(pool, func(conn pdb.DBTX) tenantdomain.Repository {
		return tenantadapters.NewRepository(conn)
	}, audit)
}

// seedUnitIn creates a unit inside the same org as parentOfOrg (or the shared test org when empty).
func seedUnitIn(t *testing.T, pool *pgxpool.Pool) string { return seedUnit(t, pool) }

// TestUnitIdFacetExpandsSubtree: a filter on the top unit must match people in its descendants.
func TestUnitIdFacetExpandsSubtree_Integration(t *testing.T) {
	svc, pool := newService(t, 720)
	bindMembership(t, svc, pool)
	tenantSvc := newTenantService(t, pool)
	ctx := context.Background()

	top := seedUnitIn(t, pool)
	mid := seedUnitIn(t, pool)
	leaf := seedUnitIn(t, pool)
	other := seedUnitIn(t, pool) // never linked into top's subtree

	// AddEdge(child, parent, graph) — build top -> mid -> leaf in the command graph.
	if _, err := tenantSvc.AddEdge(ctx, mid, top, "command"); err != nil {
		t.Fatalf("edge mid<-top: %v", err)
	}
	if _, err := tenantSvc.AddEdge(ctx, leaf, mid, "command"); err != nil {
		t.Fatalf("edge leaf<-mid: %v", err)
	}

	atTop := seedPerson(t, svc)
	atMid := seedPerson(t, svc)
	atLeaf := seedPerson(t, svc)
	atOther := seedPerson(t, svc)
	seedMembership(t, pool, atTop, top)
	seedMembership(t, pool, atMid, mid)
	seedMembership(t, pool, atLeaf, leaf)
	seedMembership(t, pool, atOther, other)

	got := allIDs(t, func(tok string) (application.Page, error) {
		return svc.ListPersons(ctx, domain.PersonFilter{UnitID: &top}, 0, tok)
	})
	for name, id := range map[string]string{"top": atTop, "mid": atMid, "leaf": atLeaf} {
		if !got[id] {
			t.Errorf("unitId=top missed the person at %s (%s) — the closure expansion is not applied", name, id)
		}
	}
	if got[atOther] {
		t.Error("unitId=top matched a person in an unrelated unit")
	}

	// Filtering by the leaf must NOT match the ancestors: expansion is downward only.
	leafOnly := allIDs(t, func(tok string) (application.Page, error) {
		return svc.ListPersons(ctx, domain.PersonFilter{UnitID: &leaf}, 0, tok)
	})
	if !leafOnly[atLeaf] {
		t.Error("unitId=leaf missed the person in the leaf unit itself")
	}
	if leafOnly[atTop] || leafOnly[atMid] {
		t.Error("unitId=leaf matched an ANCESTOR's person — the expansion must be downward only")
	}
}

// TestUnitIdFacetGraphNarrowing: the same two units linked in two graphs. Unfiltered, the expansion
// spans every authority-bearing graph; naming one graph restricts it to that graph's closure.
func TestUnitIdFacetGraphNarrowing_Integration(t *testing.T) {
	svc, pool := newService(t, 720)
	bindMembership(t, svc, pool)
	tenantSvc := newTenantService(t, pool)
	ctx := context.Background()

	root := seedUnitIn(t, pool)
	viaCommand := seedUnitIn(t, pool)
	viaOperational := seedUnitIn(t, pool)

	if _, err := tenantSvc.AddEdge(ctx, viaCommand, root, "command"); err != nil {
		t.Fatalf("edge in command: %v", err)
	}
	if _, err := tenantSvc.AddEdge(ctx, viaOperational, root, "operational"); err != nil {
		t.Skipf("the operational graph is unavailable in this fixture: %v", err)
	}

	inCommand := seedPerson(t, svc)
	inOperational := seedPerson(t, svc)
	seedMembership(t, pool, inCommand, viaCommand)
	seedMembership(t, pool, inOperational, viaOperational)

	// Default: every authority-bearing graph, so both descendants match.
	both := allIDs(t, func(tok string) (application.Page, error) {
		return svc.ListPersons(ctx, domain.PersonFilter{UnitID: &root}, 0, tok)
	})
	if !both[inCommand] {
		t.Error("default expansion missed the command-graph descendant")
	}
	if !both[inOperational] {
		t.Error("default expansion missed the operational-graph descendant (both graphs are authority-bearing)")
	}

	// Narrowed to `command`: only the command-graph descendant survives.
	narrowed := allIDs(t, func(tok string) (application.Page, error) {
		return svc.ListPersons(ctx, domain.PersonFilter{UnitID: &root, Graph: "command"}, 0, tok)
	})
	if !narrowed[inCommand] {
		t.Error("graph=command dropped the command-graph descendant")
	}
	if narrowed[inOperational] {
		t.Error("graph=command still matched the operational-graph descendant — the narrowing is not applied")
	}
}

// TestUnitIdFacetOnScopedPath: the same expansion must happen inside the read-scope queries, not
// just on the admin path — otherwise a scoped operator filtering by a parent unit would see only
// its direct members.
func TestUnitIdFacetSubtreeOnScopedPath_Integration(t *testing.T) {
	svc, pool := newService(t, 720)
	bindMembership(t, svc, pool)
	tenantSvc := newTenantService(t, pool)
	ctx := context.Background()

	top := seedUnitIn(t, pool)
	leaf := seedUnitIn(t, pool)
	if _, err := tenantSvc.AddEdge(ctx, leaf, top, "command"); err != nil {
		t.Fatalf("edge leaf<-top: %v", err)
	}

	atLeaf := seedPerson(t, svc)
	seedMembership(t, pool, atLeaf, leaf)

	// The reader holds a SUBTREE grant on top, so the leaf person is genuinely in reach.
	reader := seedPerson(t, svc)
	seedSubtreeReadGrant(t, pool, reader, top)

	got := allIDs(t, func(tok string) (application.Page, error) {
		return svc.ListVisiblePersons(ctx, reader, domain.PersonFilter{UnitID: &top}, 0, tok)
	})
	if !got[atLeaf] {
		t.Fatal("the scoped path did not expand unitId over the closure — a filter on a parent unit " +
			"returned only its direct members")
	}
}

// seedSubtreeReadGrant is seedReadGrant with scope='subtree', so the reader's reach covers the
// unit's descendants (the unit-scope variant grants children nothing — not even read).
func seedSubtreeReadGrant(t *testing.T, pool *pgxpool.Pool, readerID, unitID string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Facet subtree role') RETURNING id`,
		code(t, "facet-subtree-role")).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, 'person.read')`,
		roleID); err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
		 SELECT $1, $2, $3, 'subtree', g.id
		 FROM oikumenea.tenant_units u
		 JOIN oikumenea.tenant_graphs g ON g.org_id = u.org_id AND g.code = 'command' AND g.deleted_at IS NULL
		 WHERE u.id = $3`,
		readerID, roleID, unitID); err != nil {
		t.Fatalf("seed subtree assignment: %v", err)
	}
}
