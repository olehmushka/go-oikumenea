// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the tenant module against a real Postgres (M3 exit criteria, D-Graphs /
// D-ClosureIntegrity / D-Audit):
//   - the graph registry is boot-seeded (command default+authority-bearing, operational);
//   - a multi-parent DAG builds and its closure answers ancestors/descendants in one lookup;
//   - a per-graph cycle attempt (and a self-loop) is rejected, while the reverse edge in a
//     different graph is allowed;
//   - a lifecycle transition is recorded + audited, and an illegal transition is rejected;
//   - closure verify reports zero drift after incremental maintenance; rebuild is a no-op then;
//   - a create write + its audit row share one transaction.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/tenant/...
package tenant_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	"github.com/olegamysk/go-oikumenea/internal/tenant/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newService(t *testing.T) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	ctx := context.Background()
	pool, err := pdb.NewPool(ctx, dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	auditSvc := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, auditSvc), pool
}

// seedOrg creates a fresh domain + organization (the realm) via the application service. Creating the
// org seeds its command + operational graphs in the same transaction (D-TenantOrganizations, M40), so
// the per-org graph registry exists for the edge/closure tests.
func seedOrg(t *testing.T, svc *application.Service) domain.Organization {
	t.Helper()
	ctx := context.Background()
	code := uuid.NewString()[:8]
	d, err := svc.CreateDomain(ctx, "dom-"+code, "Domain "+code, nil)
	if err != nil {
		t.Fatalf("create domain: %v", err)
	}
	org, err := svc.CreateOrganization(ctx, domain.Organization{Code: "org-" + code, Name: "Org " + code, DomainID: d.ID})
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}
	return org
}

func newAuditSvc(pool *pgxpool.Pool) *auditapp.Service {
	return auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
}

// uniqueCode returns a fresh unit/graph code per test run so repeated runs against a persistent DB
// don't collide on the partial-unique code index.
func uniqueCode(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
}

func mustCreate(t *testing.T, svc *application.Service, org domain.Organization, code string) domain.Unit {
	t.Helper()
	u, err := svc.CreateUnit(context.Background(), domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Code: &code, Name: code})
	if err != nil {
		t.Fatalf("create unit %q: %v", code, err)
	}
	return u
}

// TestSeededGraphs asserts the boot seed produced command (default + authority-bearing) + operational.
func TestSeededGraphs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	graphs, err := svc.ListGraphs(ctx, &org.ID)
	if err != nil {
		t.Fatalf("list graphs: %v", err)
	}
	byCode := make(map[string]domain.Graph)
	for _, g := range graphs {
		byCode[g.Code] = g
	}
	cmd, ok := byCode["command"]
	if !ok || !cmd.IsDefault || !cmd.IsAuthorityBearing {
		t.Fatalf("expected command default+authority-bearing, got %+v (present=%v)", cmd, ok)
	}
	op, ok := byCode["operational"]
	if !ok || op.IsDefault || !op.IsAuthorityBearing {
		t.Fatalf("expected operational non-default authority-bearing, got %+v (present=%v)", op, ok)
	}
}

// TestCreateUnitWritesAuditRow is the headline D-Audit guarantee: the unit and its audit row are
// both readable after the write, sharing one transaction.
func TestCreateUnitWritesAuditRow(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	u := mustCreate(t, svc, org, uniqueCode(t, "unit"))

	got, err := svc.GetUnit(ctx, u.ID)
	if err != nil {
		t.Fatalf("get unit: %v", err)
	}
	if derefStr(got.Code) != derefStr(u.Code) || got.State != domain.StateActive || got.Visibility != domain.VisibilityPublic {
		t.Fatalf("unexpected unit: %+v", got)
	}

	tt := "unit"
	page, err := newAuditSvc(pool).Query(ctx, auditapp.QueryParams{TargetType: &tt, TargetID: &u.ID})
	if err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected 1 audit row for the new unit, got %d", len(page.Entries))
	}
	e := page.Entries[0]
	if e.ActorType != auditdomain.ActorSystem || e.Subsystem != "tenant-admin" || e.Action != "unit.create" {
		t.Fatalf("unexpected audit entry: %+v", e)
	}
}

// TestMultiParentDAGAndClosure builds a diamond (d has two parents) and checks the closure answers.
func TestMultiParentDAGAndClosure(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	c := mustCreate(t, svc, org, uniqueCode(t, "c"))
	d := mustCreate(t, svc, org, uniqueCode(t, "d"))

	// command edges: a->b, a->c, b->d, c->d (AddEdge(child, parent, graph)).
	for _, e := range [][2]string{{b.ID, a.ID}, {c.ID, a.ID}, {d.ID, b.ID}, {d.ID, c.ID}} {
		if _, err := svc.AddEdge(ctx, e[0], e[1], "command"); err != nil {
			t.Fatalf("add edge %v: %v", e, err)
		}
	}

	ancestors, err := svc.Ancestors(ctx, d.ID, "command")
	if err != nil {
		t.Fatalf("ancestors: %v", err)
	}
	gotAnc := idDepth(ancestors)
	if len(gotAnc) != 3 || gotAnc[a.ID] != 2 || gotAnc[b.ID] != 1 || gotAnc[c.ID] != 1 {
		t.Fatalf("unexpected ancestors of d: %+v", gotAnc)
	}

	desc, err := svc.Descendants(ctx, a.ID, "command", 0, "")
	if err != nil {
		t.Fatalf("descendants: %v", err)
	}
	gotDesc := idDepth(desc.Refs)
	if len(gotDesc) != 3 || gotDesc[b.ID] != 1 || gotDesc[c.ID] != 1 || gotDesc[d.ID] != 2 {
		t.Fatalf("unexpected descendants of a: %+v", gotDesc)
	}

	// Detach one diamond arm (b->d): d keeps a as an ancestor through the surviving c arm (M48
	// incremental shrink must keep the alternative path, same depth).
	if err := svc.RemoveEdge(ctx, d.ID, b.ID, "command"); err != nil {
		t.Fatalf("remove b->d: %v", err)
	}
	ancestors, err = svc.Ancestors(ctx, d.ID, "command")
	if err != nil {
		t.Fatalf("ancestors after detach: %v", err)
	}
	gotAnc = idDepth(ancestors)
	if len(gotAnc) != 2 || gotAnc[a.ID] != 2 || gotAnc[c.ID] != 1 {
		t.Fatalf("unexpected ancestors of d after detaching b->d: %+v", gotAnc)
	}
}

// TestListRootsAndChildren proves the hierarchy-browsing modes of ListUnits: rootsOnly returns the
// org's top-level units (no parent edge in the graph) and parent returns one unit's DIRECT children.
func TestListRootsAndChildren(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	c := mustCreate(t, svc, org, uniqueCode(t, "c"))
	d := mustCreate(t, svc, org, uniqueCode(t, "d"))

	// command edges: a->b, a->c, b->d (AddEdge(child, parent, graph)).
	for _, e := range [][2]string{{b.ID, a.ID}, {c.ID, a.ID}, {d.ID, b.ID}} {
		if _, err := svc.AddEdge(ctx, e[0], e[1], "command"); err != nil {
			t.Fatalf("add edge %v: %v", e, err)
		}
	}

	ids := func(p application.UnitPage) map[string]bool {
		m := map[string]bool{}
		for _, u := range p.Units {
			m[u.ID] = true
		}
		return m
	}

	// rootsOnly: only `a` has no parent edge in command.
	roots, err := svc.ListUnits(ctx, org.ID, nil, nil, nil, "command", nil, true, 0, "")
	if err != nil {
		t.Fatalf("list roots: %v", err)
	}
	gotRoots := ids(roots)
	if len(gotRoots) != 1 || !gotRoots[a.ID] {
		t.Fatalf("unexpected roots: %+v (want only %s)", gotRoots, a.ID)
	}

	// parent=a: direct children are b and c (NOT d, which is a grandchild).
	kids, err := svc.ListUnits(ctx, org.ID, nil, nil, nil, "command", &a.ID, false, 0, "")
	if err != nil {
		t.Fatalf("list children of a: %v", err)
	}
	gotKids := ids(kids)
	if len(gotKids) != 2 || !gotKids[b.ID] || !gotKids[c.ID] {
		t.Fatalf("unexpected children of a: %+v (want %s, %s)", gotKids, b.ID, c.ID)
	}

	// parent=b: direct child is d only.
	bKids, err := svc.ListUnits(ctx, org.ID, nil, nil, nil, "command", &b.ID, false, 0, "")
	if err != nil {
		t.Fatalf("list children of b: %v", err)
	}
	if gotBKids := ids(bKids); len(gotBKids) != 1 || !gotBKids[d.ID] {
		t.Fatalf("unexpected children of b: %+v (want %s)", gotBKids, d.ID)
	}

	// parent and rootsOnly are mutually exclusive.
	if _, err := svc.ListUnits(ctx, org.ID, nil, nil, nil, "command", &a.ID, true, 0, ""); !errors.Is(err, domain.ErrInvalidUnit) {
		t.Fatalf("expected ErrInvalidUnit for parent+rootsOnly, got %v", err)
	}
}

// TestCycleRejectedAndCrossGraphAllowed proves per-graph DAG enforcement and cross-graph freedom.
func TestCycleRejectedAndCrossGraphAllowed(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))

	// command: a->b.
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("add a->b: %v", err)
	}
	// command: b->a would close a cycle.
	if _, err := svc.AddEdge(ctx, a.ID, b.ID, "command"); !errors.Is(err, domain.ErrUnitCycle) {
		t.Fatalf("expected ErrUnitCycle for b->a in command, got %v", err)
	}
	// self-loop is the degenerate cycle.
	if _, err := svc.AddEdge(ctx, a.ID, a.ID, "command"); !errors.Is(err, domain.ErrUnitCycle) {
		t.Fatalf("expected ErrUnitCycle for self-loop, got %v", err)
	}
	// operational: b->a is legal (a different graph; cross-graph cycles are allowed).
	if _, err := svc.AddEdge(ctx, a.ID, b.ID, "operational"); err != nil {
		t.Fatalf("expected b->a allowed in operational, got %v", err)
	}
}

// TestTransitionRecordedAndAudited transitions a unit and checks the audit row + illegal rejection.
func TestTransitionRecordedAndAudited(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	u := mustCreate(t, svc, org, uniqueCode(t, "unit"))

	suspended, err := svc.TransitionUnit(ctx, u.ID, domain.StateSuspended, "drill")
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if suspended.State != domain.StateSuspended {
		t.Fatalf("expected suspended, got %s", suspended.State)
	}

	tt := "unit"
	action := "unit.transition"
	page, err := newAuditSvc(pool).Query(ctx, auditapp.QueryParams{TargetType: &tt, TargetID: &u.ID, Action: &action})
	if err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected 1 transition audit row, got %d", len(page.Entries))
	}

	// archived -> suspended is illegal; first archive, then attempt the illegal hop.
	if _, err := svc.TransitionUnit(ctx, u.ID, domain.StateArchived, ""); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := svc.TransitionUnit(ctx, u.ID, domain.StateSuspended, ""); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition archived->suspended, got %v", err)
	}
}

// TestVerifyAndRebuildClosure asserts incremental maintenance leaves no drift, and rebuild agrees.
func TestVerifyAndRebuildClosure(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("add edge: %v", err)
	}

	// Graph codes are per-org now (M40), so verify ALL graphs and assert none is in drift after the
	// incremental maintenance (a code filter would match every org's command graph).
	assertNoDrift := func(when string) {
		reports, err := svc.VerifyClosure(ctx, nil)
		if err != nil {
			t.Fatalf("verify %s: %v", when, err)
		}
		if len(reports) == 0 {
			t.Fatalf("expected at least one graph report %s", when)
		}
		for _, r := range reports {
			if r.InDrift || r.MissingCount != 0 || r.ExtraCount != 0 {
				t.Fatalf("expected zero drift %s, got %+v", when, r)
			}
		}
	}
	assertNoDrift("after maintenance")

	if _, err := svc.RebuildClosure(ctx, nil); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertNoDrift("after rebuild")
}

// TestRemoveEdgeUpdatesClosure detaches an edge and confirms the descendant disappears.
func TestRemoveEdgeUpdatesClosure(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	desc, err := svc.Descendants(ctx, a.ID, "command", 0, "")
	if err != nil || len(desc.Refs) != 1 {
		t.Fatalf("expected 1 descendant before remove, got %d (err=%v)", len(desc.Refs), err)
	}
	if err := svc.RemoveEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("remove edge: %v", err)
	}
	desc, err = svc.Descendants(ctx, a.ID, "command", 0, "")
	if err != nil {
		t.Fatalf("descendants after remove: %v", err)
	}
	if len(desc.Refs) != 0 {
		t.Fatalf("expected 0 descendants after remove, got %d", len(desc.Refs))
	}
}

func idDepth(refs []domain.UnitRef) map[string]int {
	m := make(map[string]int, len(refs))
	for _, r := range refs {
		m[r.ID] = r.Depth
	}
	return m
}

// seedLanguoid inserts a Glottolog language node (D-Languages, M18) and returns its RID, for the unit
// official/working-language link tests.
func seedLanguoid(t *testing.T, pool *pgxpool.Pool, code, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.language_languoids (code, level, name) VALUES ($1,'language',$2) RETURNING id`,
		code, name).Scan(&id); err != nil {
		t.Fatalf("seed languoid %s: %v", code, err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		// A concurrent language-scheme import (other package) rebuilds the global closure, which adds a
		// reflexive row for this bare languoid; clear dependents before the RESTRICT delete.
		_, _ = pool.Exec(ctx, "DELETE FROM oikumenea.language_languoid_closure WHERE ancestor_id = $1 OR descendant_id = $1", id)
		_, _ = pool.Exec(ctx, "DELETE FROM oikumenea.tenant_unit_languages WHERE language_id = $1", id)
		_, _ = pool.Exec(ctx, "DELETE FROM oikumenea.language_languoids WHERE id = $1", id)
	})
	return id
}

// TestUnitLanguages covers the unit official/working-language link (D-Languages, M18): add, the
// (unit, language) upsert key with the isOfficial flip, the unknown-language FK, and delete.
func TestUnitLanguages(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	suffix := uuid.NewString()[:8]
	lang := seedLanguoid(t, pool, "ua"+suffix[:6], "Testish")
	u := mustCreate(t, svc, org, "unit-lang-"+suffix)

	saved, err := svc.UpsertUnitLanguage(ctx, domain.UnitLanguage{UnitID: u.ID, LanguageID: lang, IsOfficial: true})
	if err != nil {
		t.Fatalf("upsert unit language: %v", err)
	}
	if saved.LanguageName != "Testish" || !saved.IsOfficial {
		t.Fatalf("saved = %+v, want name=Testish official", saved)
	}

	// upsert keyed on (unit, language): flip to working language, no duplicate row
	if _, err := svc.UpsertUnitLanguage(ctx, domain.UnitLanguage{UnitID: u.ID, LanguageID: lang, IsOfficial: false}); err != nil {
		t.Fatalf("update unit language: %v", err)
	}
	ls, err := svc.ListUnitLanguages(ctx, u.ID)
	if err != nil || len(ls) != 1 {
		t.Fatalf("list after update: len=%d err=%v", len(ls), err)
	}
	if ls[0].IsOfficial {
		t.Fatalf("expected working language (isOfficial=false), got %+v", ls[0])
	}

	// an unresolved languoid trips the FK -> ErrUnknownLanguage
	var bogus string
	if err := pool.QueryRow(ctx, "SELECT oikumenea.new_id(13,1,1)").Scan(&bogus); err != nil {
		t.Fatalf("mint bogus languoid rid: %v", err)
	}
	if _, err := svc.UpsertUnitLanguage(ctx, domain.UnitLanguage{UnitID: u.ID, LanguageID: bogus}); !errors.Is(err, domain.ErrUnknownLanguage) {
		t.Fatalf("unknown languoid should be ErrUnknownLanguage, got %v", err)
	}

	if err := svc.DeleteUnitLanguage(ctx, u.ID, lang); err != nil {
		t.Fatalf("delete unit language: %v", err)
	}
	if ls, err := svc.ListUnitLanguages(ctx, u.ID); err != nil || len(ls) != 0 {
		t.Fatalf("list after delete: len=%d err=%v", len(ls), err)
	}
}

// TestUnitCodeLifecycle exercises D-UnitCodeLifecycle (M28): codeless creation + coexisting codeless
// siblings, the audited set/correct/clear recode path with its append-only ledger, and the active
// uniqueness conflict.
func TestUnitCodeLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	// codeless create succeeds, and two codeless siblings coexist (the partial-unique index ignores NULLs).
	a, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "3rd Platoon"})
	if err != nil {
		t.Fatalf("create codeless unit a: %v", err)
	}
	if a.Code != nil {
		t.Fatalf("expected codeless unit, got code=%v", *a.Code)
	}
	b, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "4th Platoon"})
	if err != nil {
		t.Fatalf("create codeless sibling b: %v", err)
	}

	// set a code on the codeless unit a.
	code1 := uniqueCode(t, "bn")
	got, err := svc.SetUnitCode(ctx, a.ID, &code1, "initial designation")
	if err != nil {
		t.Fatalf("set code: %v", err)
	}
	if derefStr(got.Code) != code1 {
		t.Fatalf("expected code %q, got %q", code1, derefStr(got.Code))
	}

	// a second unit taking the same active code -> conflict.
	if _, err := svc.SetUnitCode(ctx, b.ID, &code1, ""); !errors.Is(err, domain.ErrUnitCodeConflict) {
		t.Fatalf("duplicate active code should be ErrUnitCodeConflict, got %v", err)
	}

	// correct the code (value -> value), then clear it (value -> NULL).
	code2 := uniqueCode(t, "bn")
	if _, err := svc.SetUnitCode(ctx, a.ID, &code2, "reorg"); err != nil {
		t.Fatalf("correct code: %v", err)
	}
	cleared, err := svc.SetUnitCode(ctx, a.ID, nil, "became non-separate")
	if err != nil {
		t.Fatalf("clear code: %v", err)
	}
	if cleared.Code != nil {
		t.Fatalf("expected cleared code, got %v", *cleared.Code)
	}

	// the ledger holds one event per change, newest first: clear (code2->nil), correct (code1->code2), set (nil->code1).
	events, err := svc.ListUnitCodeEvents(ctx, a.ID)
	if err != nil {
		t.Fatalf("list code events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 code events, got %d", len(events))
	}
	if events[0].OldCode == nil || derefStr(events[0].OldCode) != code2 || events[0].NewCode != nil {
		t.Fatalf("newest event should be code2->nil, got old=%v new=%v", events[0].OldCode, events[0].NewCode)
	}
	if events[2].OldCode != nil || derefStr(events[2].NewCode) != code1 {
		t.Fatalf("oldest event should be nil->code1, got old=%v new=%v", events[2].OldCode, events[2].NewCode)
	}
	for _, e := range events {
		if e.RequestID == "" {
			t.Fatalf("code event missing request id: %+v", e)
		}
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
