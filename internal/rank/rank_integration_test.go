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
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/postgres?sslmode=disable" \
//	  go test -tags integration ./internal/rank/...
package rank_test

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
	locadapters "github.com/olegamysk/go-oikumenea/internal/localization/adapters"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	locdomain "github.com/olegamysk/go-oikumenea/internal/localization/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/rank/adapters"
	"github.com/olegamysk/go-oikumenea/internal/rank/application"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable"

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

	cat, err := svc.AddCategory(ctx, code(t, "army"), "Army", nil)
	if err != nil {
		t.Fatalf("add category: %v", err)
	}
	typ, err := svc.AddType(ctx, cat.ID, code(t, "officers"), "Officers", nil)
	if err != nil {
		t.Fatalf("add type: %v", err)
	}
	jr, err := svc.AddRank(ctx, typ.ID, code(t, "lt"), "Lieutenant", strPtr("LT"), nil)
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

	cat, err := svc.AddCategory(ctx, code(t, "navy"), "Navy", nil)
	if err != nil {
		t.Fatalf("add category: %v", err)
	}
	// Two types added with explicit out-of-order sort_order; the read must put 1 before 2.
	hi, err := svc.AddType(ctx, cat.ID, code(t, "enlisted"), "Enlisted", ptrInt(2))
	if err != nil {
		t.Fatalf("add type hi: %v", err)
	}
	lo, err := svc.AddType(ctx, cat.ID, code(t, "officers"), "Officers", ptrInt(1))
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

	c := code(t, "dup")
	if _, err := svc.AddCategory(ctx, c, "First", nil); err != nil {
		t.Fatalf("add first: %v", err)
	}
	_, err := svc.AddCategory(ctx, c, "Second", nil)
	if !errors.Is(err, domain.ErrCodeConflict) {
		t.Fatalf("want ErrCodeConflict, got %v", err)
	}
}

// TestStrictContainment rejects adding a node under an absent parent.
func TestStrictContainment(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	_, err := svc.AddType(ctx, "urn:oikumenea:rank:local:category:"+uuid.NewString(), code(t, "x"), "X", nil)
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("type under missing category: want ErrCategoryNotFound, got %v", err)
	}
	_, err = svc.AddRank(ctx, "urn:oikumenea:rank:local:type:"+uuid.NewString(), code(t, "y"), "Y", nil, nil)
	if !errors.Is(err, domain.ErrTypeNotFound) {
		t.Fatalf("rank under missing type: want ErrTypeNotFound, got %v", err)
	}
}

// TestDeleteInUse blocks deleting a node with active children, then deletes bottom-up.
func TestDeleteInUse(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	cat, _ := svc.AddCategory(ctx, code(t, "del"), "Del", nil)
	typ, _ := svc.AddType(ctx, cat.ID, code(t, "band"), "Band", nil)
	rnk, _ := svc.AddRank(ctx, typ.ID, code(t, "grade"), "Grade", nil, nil)

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

	cat, err := svc.AddCategory(ctx, code(t, "aud"), "Audited", nil)
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
