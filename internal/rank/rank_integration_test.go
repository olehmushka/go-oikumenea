//go:build integration

// Integration tests for the rank module against a real Postgres (M4 exit criteria, D-Rank /
// L-OneRankScheme / D-Audit):
//   - a populated scheme reads back as category -> type -> rank in seniority order, with localized
//     name maps assembled from the default locale;
//   - codes are unique within their parent among active nodes (duplicate -> conflict);
//   - containment is strict: adding under an absent/soft-deleted parent is rejected;
//   - a node in use cannot be deleted (category with active types, type with active ranks); deleting
//     bottom-up succeeds;
//   - reordering changes the read order;
//   - a create write + its audit row share one transaction.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/rank/...
package rank_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	rankapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/rank"
	locadapters "github.com/olegamysk/go-oikumenea/internal/localization/adapters"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	locdomain "github.com/olegamysk/go-oikumenea/internal/localization/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/rank/adapters"
	"github.com/olegamysk/go-oikumenea/internal/rank/application"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
	"github.com/olegamysk/go-oikumenea/internal/rank/transport"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newPool(t *testing.T) *pgxpool.Pool {
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
	return pool
}

func newAuditSvc(pool *pgxpool.Pool) *auditapp.Service {
	return auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
}

func newService(t *testing.T) (*application.Service, *locapp.Service, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	audit := newAuditSvc(pool)
	loc := locapp.NewService(pool, func(conn pdb.DBTX) locdomain.Repository {
		return locadapters.NewRepository(conn)
	}, audit)
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, audit), loc, pool
}

func code(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
}

func ptrInt(n int) *int { return &n }

// TestSchemeBuildAndRead populates a category -> type -> rank chain and reads it back ordered.
func TestSchemeBuildAndRead(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, err := svc.AddCategory(ctx, sys.ID, code(t, "army"), "Army", nil)
	if err != nil {
		t.Fatalf("add category: %v", err)
	}
	typ, err := svc.AddType(ctx, cat.ID, nil, code(t, "officers"), "Officers", nil)
	if err != nil {
		t.Fatalf("add type: %v", err)
	}
	jr, err := svc.AddRank(ctx, typ.ID, code(t, "lt"), "Lieutenant", strPtr("LT"), nil, nil)
	if err != nil {
		t.Fatalf("add rank: %v", err)
	}

	scheme, err := svc.GetScheme(ctx)
	if err != nil {
		t.Fatalf("get scheme: %v", err)
	}
	if !containsCategory(scheme.Categories, cat.ID) {
		t.Fatalf("scheme missing category %s", cat.ID)
	}
	// The category and its denormalized system line up, and the rank inherits the system.
	if cat.SystemID != sys.ID {
		t.Fatalf("category.SystemID = %q, want %q", cat.SystemID, sys.ID)
	}
	if typ.SystemID != sys.ID || jr.SystemID != sys.ID {
		t.Fatalf("denormalized system_id not propagated: type=%q rank=%q want %q", typ.SystemID, jr.SystemID, sys.ID)
	}
	if !containsType(scheme.Types, typ.ID, cat.ID) {
		t.Fatalf("scheme missing type %s under %s", typ.ID, cat.ID)
	}
	if !containsRank(scheme.Ranks, jr.ID, typ.ID) {
		t.Fatalf("scheme missing rank %s under %s", jr.ID, typ.ID)
	}
	if jr.Abbreviation != "LT" {
		t.Fatalf("abbreviation = %q, want LT", jr.Abbreviation)
	}
}

// TestSeniorityOrdering inserts categories out of order and confirms the read is sorted by sort_order.
func TestSeniorityOrdering(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, err := svc.AddCategory(ctx, sys.ID, code(t, "navy"), "Navy", nil)
	if err != nil {
		t.Fatalf("add category: %v", err)
	}
	// Two types added with explicit out-of-order sort_order; the read must put 1 before 2.
	hi, err := svc.AddType(ctx, cat.ID, nil, code(t, "enlisted"), "Enlisted", ptrInt(2))
	if err != nil {
		t.Fatalf("add type hi: %v", err)
	}
	lo, err := svc.AddType(ctx, cat.ID, nil, code(t, "officers"), "Officers", ptrInt(1))
	if err != nil {
		t.Fatalf("add type lo: %v", err)
	}

	if pos(t, svc, cat.ID, lo.ID) >= pos(t, svc, cat.ID, hi.ID) {
		t.Fatalf("expected sort_order 1 (%s) before sort_order 2 (%s)", lo.ID, hi.ID)
	}

	// Reorder: move hi to the front (sort_order 0).
	if _, err := svc.UpdateType(ctx, hi.ID, domain.TypePatch{SortOrder: ptrInt(0)}); err != nil {
		t.Fatalf("reorder type: %v", err)
	}
	if pos(t, svc, cat.ID, hi.ID) >= pos(t, svc, cat.ID, lo.ID) {
		t.Fatalf("after reorder, %s should precede %s", hi.ID, lo.ID)
	}
}

// TestCodeConflict rejects a duplicate code at the same level within a parent.
func TestCodeConflict(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	c := code(t, "dup")
	if _, err := svc.AddCategory(ctx, sys.ID, c, "First", nil); err != nil {
		t.Fatalf("add first: %v", err)
	}
	_, err := svc.AddCategory(ctx, sys.ID, c, "Second", nil)
	if !errors.Is(err, domain.ErrCodeConflict) {
		t.Fatalf("want ErrCodeConflict, got %v", err)
	}
}

// TestStrictContainment rejects adding a node under an absent parent.
func TestStrictContainment(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	_, err := svc.AddType(ctx, "urn:oikumenea:rank:local:category:"+uuid.NewString(), nil, code(t, "x"), "X", nil)
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("type under missing category: want ErrCategoryNotFound, got %v", err)
	}
	missingParent := "urn:oikumenea:rank:local:type:" + uuid.NewString()
	_, err = svc.AddType(ctx, "", &missingParent, code(t, "x"), "X", nil)
	if !errors.Is(err, domain.ErrTypeNotFound) {
		t.Fatalf("type under missing parent type: want ErrTypeNotFound, got %v", err)
	}
	_, err = svc.AddRank(ctx, "urn:oikumenea:rank:local:type:"+uuid.NewString(), code(t, "y"), "Y", nil, nil, nil)
	if !errors.Is(err, domain.ErrTypeNotFound) {
		t.Fatalf("rank under missing type: want ErrTypeNotFound, got %v", err)
	}
}

// TestDeleteInUse blocks deleting a node with active children, then deletes bottom-up.
func TestDeleteInUse(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, _ := svc.AddCategory(ctx, sys.ID, code(t, "del"), "Del", nil)
	typ, _ := svc.AddType(ctx, cat.ID, nil, code(t, "band"), "Band", nil)
	rnk, _ := svc.AddRank(ctx, typ.ID, code(t, "grade"), "Grade", nil, nil, nil)

	if err := svc.DeleteNode(ctx, domain.LevelCategory, cat.ID); !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("delete category in use: want ErrInUse, got %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelType, typ.ID); !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("delete type in use: want ErrInUse, got %v", err)
	}
	// Bottom-up succeeds.
	if err := svc.DeleteNode(ctx, domain.LevelRank, rnk.ID); err != nil {
		t.Fatalf("delete rank: %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelType, typ.ID); err != nil {
		t.Fatalf("delete type: %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelCategory, cat.ID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	// A deleted node is gone (not found).
	if err := svc.DeleteNode(ctx, domain.LevelCategory, cat.ID); !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("re-delete category: want ErrCategoryNotFound, got %v", err)
	}
}

// TestWriteAuditsInOneTx confirms a create records exactly one audit row keyed to it.
func TestWriteAuditsInOneTx(t *testing.T) {
	ctx := context.Background()
	svc, _, pool := newService(t)

	sys := addSystem(t, svc)
	cat, err := svc.AddCategory(ctx, sys.ID, code(t, "aud"), "Audited", nil)
	if err != nil {
		t.Fatalf("add category: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = $2 AND actor_type = 'system' AND subsystem = 'rank-admin'",
		cat.ID, "rank.category.create",
	).Scan(&n); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit rows for %s = %d, want 1", cat.ID, n)
	}
}

// TestTypeTree nests a child type under a parent type and confirms it inherits the parent's category
// and carries the parent link, and that a rank attaches to the leaf child.
func TestTypeTree(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, err := svc.AddCategory(ctx, sys.ID, code(t, "army"), "Army", nil)
	if err != nil {
		t.Fatalf("add category: %v", err)
	}
	parent, err := svc.AddType(ctx, cat.ID, nil, code(t, "officers"), "Officers", nil)
	if err != nil {
		t.Fatalf("add parent type: %v", err)
	}
	child, err := svc.AddType(ctx, "", strPtr(parent.ID), code(t, "junior"), "Junior officers", nil)
	if err != nil {
		t.Fatalf("add child type: %v", err)
	}
	if child.ParentTypeID != parent.ID {
		t.Fatalf("child.ParentTypeID = %q, want %q", child.ParentTypeID, parent.ID)
	}
	if child.CategoryID != cat.ID {
		t.Fatalf("child.CategoryID = %q, want inherited %q", child.CategoryID, cat.ID)
	}
	if child.SystemID != sys.ID {
		t.Fatalf("child.SystemID = %q, want inherited %q", child.SystemID, sys.ID)
	}
	// A rank attaches to the leaf child.
	if _, err := svc.AddRank(ctx, child.ID, code(t, "lt"), "Lieutenant", strPtr("LT"), nil, nil); err != nil {
		t.Fatalf("add rank under leaf child: %v", err)
	}
}

// TestLeafOnlyRule enforces that ranks live on leaf types: a type with ranks cannot gain children, and
// a type with children cannot gain ranks.
func TestLeafOnlyRule(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, _ := svc.AddCategory(ctx, sys.ID, code(t, "leaf"), "Leaf", nil)

	// A type that already holds a rank cannot gain a child type.
	withRank, _ := svc.AddType(ctx, cat.ID, nil, code(t, "withrank"), "WithRank", nil)
	if _, err := svc.AddRank(ctx, withRank.ID, code(t, "g1"), "Grade 1", nil, nil, nil); err != nil {
		t.Fatalf("add rank: %v", err)
	}
	if _, err := svc.AddType(ctx, "", strPtr(withRank.ID), code(t, "sub"), "Sub", nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("add child under type-with-ranks: want ErrInvalid, got %v", err)
	}

	// A type that already has a child type cannot gain a rank.
	withChild, _ := svc.AddType(ctx, cat.ID, nil, code(t, "withchild"), "WithChild", nil)
	if _, err := svc.AddType(ctx, "", strPtr(withChild.ID), code(t, "sub2"), "Sub2", nil); err != nil {
		t.Fatalf("add child type: %v", err)
	}
	if _, err := svc.AddRank(ctx, withChild.ID, code(t, "g2"), "Grade 2", nil, nil, nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("add rank under type-with-children: want ErrInvalid, got %v", err)
	}
}

// TestDeleteTypeWithChildren blocks deleting a type that has active child types, then succeeds
// bottom-up.
func TestDeleteTypeWithChildren(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, _ := svc.AddCategory(ctx, sys.ID, code(t, "deltree"), "DelTree", nil)
	parent, _ := svc.AddType(ctx, cat.ID, nil, code(t, "p"), "Parent", nil)
	child, _ := svc.AddType(ctx, "", strPtr(parent.ID), code(t, "c"), "Child", nil)

	if err := svc.DeleteNode(ctx, domain.LevelType, parent.ID); !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("delete parent with child: want ErrInUse, got %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelType, child.ID); err != nil {
		t.Fatalf("delete child: %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelType, parent.ID); err != nil {
		t.Fatalf("delete parent after child gone: %v", err)
	}
}

// TestSiblingCodeUniqueness rejects a duplicate code among siblings (same parent) but allows the same
// code under different parents.
func TestSiblingCodeUniqueness(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, _ := svc.AddCategory(ctx, sys.ID, code(t, "uniq"), "Uniq", nil)
	a, _ := svc.AddType(ctx, cat.ID, nil, code(t, "a"), "A", nil)
	b, _ := svc.AddType(ctx, cat.ID, nil, code(t, "b"), "B", nil)

	dup := code(t, "shared")
	if _, err := svc.AddType(ctx, "", strPtr(a.ID), dup, "Under A", nil); err != nil {
		t.Fatalf("first child: %v", err)
	}
	// Same code under a different parent (b) is allowed.
	if _, err := svc.AddType(ctx, "", strPtr(b.ID), dup, "Under B", nil); err != nil {
		t.Fatalf("same code under different parent: %v", err)
	}
	// Same code under the same parent (a) conflicts.
	if _, err := svc.AddType(ctx, "", strPtr(a.ID), dup, "Under A again", nil); !errors.Is(err, domain.ErrCodeConflict) {
		t.Fatalf("dup code under same parent: want ErrCodeConflict, got %v", err)
	}
}

// ---------------------------------------------------------------- M15: systems, grades, import

// addSystem creates a fresh rank system for a test and returns it.
func addSystem(t *testing.T, svc *application.Service) domain.System {
	t.Helper()
	sys, err := svc.AddSystem(context.Background(), code(t, "sys"), "System", nil, nil)
	if err != nil {
		t.Fatalf("add system: %v", err)
	}
	return sys
}

// TestSystemDeleteInUse blocks deleting a system that has active categories, then succeeds once empty.
func TestSystemDeleteInUse(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	sys := addSystem(t, svc)
	cat, err := svc.AddCategory(ctx, sys.ID, code(t, "army"), "Army", nil)
	if err != nil {
		t.Fatalf("add category: %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelSystem, sys.ID); !errors.Is(err, domain.ErrInUse) {
		t.Fatalf("delete system in use: want ErrInUse, got %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelCategory, cat.ID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	if err := svc.DeleteNode(ctx, domain.LevelSystem, sys.ID); err != nil {
		t.Fatalf("delete empty system: %v", err)
	}
}

// TestGradesCatalog confirms the migration-seeded STANAG 2116 catalog reads back, ordered by the
// comparability scale (enlisted -> warrant -> officer, then ordinal), and that grade_code is validated.
func TestGradesCatalog(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	grades, err := svc.GetGrades(ctx)
	if err != nil {
		t.Fatalf("get grades: %v", err)
	}
	byCode := make(map[string]domain.Grade, len(grades))
	for _, g := range grades {
		byCode[g.Code] = g
	}
	for _, c := range []string{"OF-1", "OF-5", "OF-10", "OF(D)", "OR-1", "OR-9", "WO-1"} {
		if _, ok := byCode[c]; !ok {
			t.Fatalf("seeded grade %q missing from catalog", c)
		}
	}
	// Ordered: the first grade is enlisted, the last is officer.
	if len(grades) == 0 || grades[0].Tier != domain.TierEnlisted || grades[len(grades)-1].Tier != domain.TierOfficer {
		t.Fatalf("grades not ordered by tier: first=%v last=%v", grades[0].Tier, grades[len(grades)-1].Tier)
	}

	// A rank may carry a valid grade; an unknown grade is rejected.
	sys := addSystem(t, svc)
	cat, _ := svc.AddCategory(ctx, sys.ID, code(t, "army"), "Army", nil)
	typ, _ := svc.AddType(ctx, cat.ID, nil, code(t, "officers"), "Officers", nil)
	if _, err := svc.AddRank(ctx, typ.ID, code(t, "col"), "Colonel", strPtr("COL"), strPtr("OF-5"), nil); err != nil {
		t.Fatalf("add rank with valid grade: %v", err)
	}
	if _, err := svc.AddRank(ctx, typ.ID, code(t, "bad"), "Bad", nil, strPtr("OF-99"), nil); !errors.Is(err, domain.ErrGradeNotFound) {
		t.Fatalf("add rank with unknown grade: want ErrGradeNotFound, got %v", err)
	}
}

// TestImportIdempotentAndEquivalence imports two presets, confirms a person could hold a rank in either
// system, that re-import is a no-op (all skipped), and that ranks sharing a grade_code are equivalent
// across systems while seniority compares via tier+ordinal.
func TestImportIdempotentAndEquivalence(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	us := loadPreset(t, "us-armed-forces")
	ua := loadPreset(t, "ua-armed-forces")

	// Use a unique system code per run so repeated test runs against a shared DB stay isolated.
	usCode, uaCode := code(t, "us"), code(t, "ua")
	us.System.Code, ua.System.Code = usCode, uaCode

	sumUS, err := svc.ImportPreset(ctx, us)
	if err != nil {
		t.Fatalf("import us: %v", err)
	}
	if sumUS.Created == 0 || sumUS.Updated != 0 {
		t.Fatalf("first import should be all-created: %+v", sumUS)
	}
	if _, err := svc.ImportPreset(ctx, ua); err != nil {
		t.Fatalf("import ua: %v", err)
	}

	// Re-importing the unchanged US preset changes nothing.
	reUS, err := svc.ImportPreset(ctx, us)
	if err != nil {
		t.Fatalf("re-import us: %v", err)
	}
	if reUS.Created != 0 || reUS.Updated != 0 || reUS.Skipped == 0 {
		t.Fatalf("re-import should be all-skipped: %+v", reUS)
	}

	// Find the OF-5 rank (Colonel / Полковник) in each imported system and confirm cross-system
	// equivalence + that IsSenior over the shared grade reports them equal (not strictly senior).
	scheme, err := svc.GetScheme(ctx)
	if err != nil {
		t.Fatalf("get scheme: %v", err)
	}
	grades := gradeIndex(t, svc)
	usCol := findGradedRank(t, scheme, usCode, "OF-5")
	uaCol := findGradedRank(t, scheme, uaCode, "OF-5")
	if usCol.SystemID == uaCol.SystemID {
		t.Fatal("expected the two OF-5 ranks to live in different systems")
	}
	if !domain.Equivalent(grades[usCol.GradeCode], grades[uaCol.GradeCode]) {
		t.Fatalf("US %s and UA %s should be equivalent (both OF-5)", usCol.Code, uaCol.Code)
	}
	if senior, known := domain.IsSenior(grades[usCol.GradeCode], grades[uaCol.GradeCode]); !known || senior {
		t.Fatalf("two OF-5 ranks should compare equal (known, not senior): senior=%v known=%v", senior, known)
	}
	// A UA general (OF-9) outranks the US colonel (OF-5) across systems.
	uaGen := findGradedRank(t, scheme, uaCode, "OF-9")
	if senior, known := domain.IsSenior(grades[uaGen.GradeCode], grades[usCol.GradeCode]); !known || !senior {
		t.Fatalf("UA OF-9 should be senior to US OF-5: senior=%v known=%v", senior, known)
	}
}

func loadPreset(t *testing.T, name string) application.Preset {
	t.Helper()
	raw, err := os.ReadFile("../../deploy/rank-presets/" + name + ".json")
	if err != nil {
		t.Fatalf("read preset %s: %v", name, err)
	}
	var req rankapi.ImportRankSchemeRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("parse preset %s: %v", name, err)
	}
	return transport.PresetFromRequest(req)
}

// gradeIndex returns the seeded grades keyed by code, for IsSenior/Equivalent lookups in tests.
func gradeIndex(t *testing.T, svc *application.Service) map[string]domain.Grade {
	t.Helper()
	grades, err := svc.GetGrades(context.Background())
	if err != nil {
		t.Fatalf("get grades: %v", err)
	}
	m := make(map[string]domain.Grade, len(grades))
	for _, g := range grades {
		m[g.Code] = g
	}
	return m
}

// findGradedRank walks the scheme for the first active rank in the given system code carrying gradeCode.
func findGradedRank(t *testing.T, scheme application.Scheme, systemCode, gradeCode string) domain.Rank {
	t.Helper()
	var systemID string
	for _, sys := range scheme.Systems {
		if sys.Code == systemCode {
			systemID = sys.ID
			break
		}
	}
	if systemID == "" {
		t.Fatalf("system %q not found in scheme", systemCode)
	}
	for _, r := range scheme.Ranks {
		if r.SystemID == systemID && r.GradeCode == gradeCode {
			return r
		}
	}
	t.Fatalf("no rank with grade %q in system %q", gradeCode, systemCode)
	return domain.Rank{}
}

// ---- helpers ----

func strPtr(s string) *string { return &s }

func pos(t *testing.T, svc *application.Service, categoryID, typeID string) int {
	t.Helper()
	scheme, err := svc.GetScheme(context.Background())
	if err != nil {
		t.Fatalf("get scheme: %v", err)
	}
	i := 0
	for _, ty := range scheme.Types {
		if ty.CategoryID != categoryID {
			continue
		}
		if ty.ID == typeID {
			return i
		}
		i++
	}
	t.Fatalf("type %s not found under category %s", typeID, categoryID)
	return -1
}

func containsCategory(cs []domain.Category, id string) bool {
	for _, c := range cs {
		if c.ID == id {
			return true
		}
	}
	return false
}

func containsType(ts []domain.Type, id, categoryID string) bool {
	for _, t := range ts {
		if t.ID == id && t.CategoryID == categoryID {
			return true
		}
	}
	return false
}

func containsRank(rs []domain.Rank, id, typeID string) bool {
	for _, r := range rs {
		if r.ID == id && r.TypeID == typeID {
			return true
		}
	}
	return false
}
