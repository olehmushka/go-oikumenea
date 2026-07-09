//go:build integration

// Integration tests for the External-organizations vertical against a real Postgres (M30 exit criteria,
// D-ExternalOrgs / D-Audit). They exercise the audited kind-catalog + org CRUD, the exit example
// (a political party and a government ministry as distinct kinds), the wikidata-id/code conflict guards,
// the provisional→resolved merge (stub tombstoned, canonical survives), and the hermenea import handler's
// idempotent upsert (create → re-run skips → changed name updates → unknown kind skipped).
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/externalorg/...
package externalorg_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	dataimportadapters "github.com/olegamysk/go-oikumenea/internal/dataimport/adapters"
	dataimportapp "github.com/olegamysk/go-oikumenea/internal/dataimport/application"
	dataimportdomain "github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/externalorg/adapters"
	"github.com/olegamysk/go-oikumenea/internal/externalorg/application"
	"github.com/olegamysk/go-oikumenea/internal/externalorg/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
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

func newService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	}, audit)
}

func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func kindID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.external_org_kinds WHERE code = $1 AND deleted_at IS NULL", code).Scan(&id); err != nil {
		t.Fatalf("resolve kind %s: %v", code, err)
	}
	return id
}

func uaCountryID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.geo_countries WHERE code = 'UA'").Scan(&id); err != nil {
		t.Fatalf("resolve UA: %v", err)
	}
	return id
}

// TestKindCatalogAndCreate proves the seeded kind catalog reads, and a party + a government ministry
// register as distinct kinds (the M30 exit example), with the kind filter selecting just one.
func TestKindCatalogAndCreate(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)

	kinds, err := svc.ListKinds(ctx)
	if err != nil {
		t.Fatalf("ListKinds: %v", err)
	}
	seen := map[string]bool{}
	for _, k := range kinds {
		seen[k.Code] = true
	}
	for _, want := range []string{"party", "government_body", "military", "ngo", "registrant", "other"} {
		if !seen[want] {
			t.Fatalf("seeded kind %q missing from catalog", want)
		}
	}

	party, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: kindID(t, pool, "party"), Name: uniq("Servant of the People"), CountryID: uaCountryID(t, pool)})
	if err != nil {
		t.Fatalf("create party: %v", err)
	}
	ministry, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: kindID(t, pool, "government_body"), Name: uniq("Ministry of Defence")})
	if err != nil {
		t.Fatalf("create ministry: %v", err)
	}
	if party.KindID == ministry.KindID {
		t.Fatal("party and ministry should have distinct kinds")
	}
	if party.Status != domain.StatusResolved || party.Source != domain.SourceOperatorVerified {
		t.Fatalf("defaults: status=%s source=%s", party.Status, party.Source)
	}

	got, err := svc.ListOrgs(ctx, "", "party", "", "", "", 100)
	if err != nil {
		t.Fatalf("ListOrgs(kind=party): %v", err)
	}
	for _, o := range got {
		if o.KindID != party.KindID {
			t.Fatalf("kind filter leaked a non-party org %s", o.ID)
		}
	}
}

// TestConflictGuards proves the wikidata-id and code unique-active guards.
func TestConflictGuards(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	party := kindID(t, pool, "party")

	wd := uniq("Q")
	if _, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: party, Name: uniq("A"), WikidataID: wd}); err != nil {
		t.Fatalf("create #1: %v", err)
	}
	if _, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: party, Name: uniq("B"), WikidataID: wd}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate wikidata id: want ErrConflict, got %v", err)
	}

	code := uniq("code")
	if _, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: party, Name: uniq("C"), Code: code}); err != nil {
		t.Fatalf("create #3: %v", err)
	}
	if _, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: party, Name: uniq("D"), Code: code}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate code: want ErrConflict, got %v", err)
	}
}

// TestProvisionalMergeResolves proves a provisional stub resolves into a canonical org: the stub is
// tombstoned (gone from reads), the canonical survives, and an invalid merge is rejected.
func TestProvisionalMergeResolves(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	party := kindID(t, pool, "party")

	canonical, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: party, Name: uniq("Canonical Party")})
	if err != nil {
		t.Fatalf("create canonical: %v", err)
	}
	stub, err := svc.CreateOrg(ctx, domain.OrgInput{KindID: party, Name: uniq("Stub Party"), Status: domain.StatusProvisional})
	if err != nil {
		t.Fatalf("create stub: %v", err)
	}
	if stub.Status != domain.StatusProvisional {
		t.Fatalf("stub status = %s", stub.Status)
	}

	// Merging a resolved org into another is rejected (only a provisional stub may be the merge source).
	if _, err := svc.MergeOrg(ctx, canonical.ID, stub.ID, ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("merge resolved-into-provisional: want ErrInvalid, got %v", err)
	}

	out, err := svc.MergeOrg(ctx, stub.ID, canonical.ID, domain.ConfidenceConfirmed)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if out.ID != canonical.ID {
		t.Fatalf("merge returned %s, want canonical %s", out.ID, canonical.ID)
	}
	if _, err := svc.GetOrg(ctx, stub.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("stub should be tombstoned: got %v", err)
	}
	if _, err := svc.GetOrg(ctx, canonical.ID); err != nil {
		t.Fatalf("canonical should survive: %v", err)
	}
}

// TestImportHandlerIdempotent drives the hermenea import handler (D-ExternalOrgs) directly over a tx:
// create → re-run skips → changed name updates → unknown kind skipped.
func TestImportHandlerIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	handler := dataimportapp.ExternalOrgsHandler(func(conn pdb.DBTX) dataimportdomain.ExternalOrgStore {
		return dataimportadapters.NewExternalOrgRepo(conn)
	})
	prov := dataimportdomain.Provenance{Source: "wikidata-orgs-ua", SourceVersion: "v1", ImportedAt: time.Now().UTC()}

	q1, q2, q3 := uniq("Q"), uniq("Q"), uniq("Q")
	recs := []dataimportdomain.Record{
		{"wikidataId": q1, "name": "Party One", "kind": "party", "country": "UA"},
		{"wikidataId": q2, "name": "Ministry Two", "kind": "government_body"},
		{"wikidataId": q3, "name": "Bad Kind", "kind": "no-such-kind"}, // unknown kind → skipped
	}

	sum := runImport(t, pool, ctx, handler, recs, prov)
	if sum.Created != 2 || sum.Skipped != 1 {
		t.Fatalf("first run: created=%d updated=%d skipped=%d (want 2/0/1)", sum.Created, sum.Updated, sum.Skipped)
	}

	// Re-run identical → all skipped (q3 still unknown kind).
	sum = runImport(t, pool, ctx, handler, recs, prov)
	if sum.Created != 0 || sum.Updated != 0 || sum.Skipped != 3 {
		t.Fatalf("re-run: created=%d updated=%d skipped=%d (want 0/0/3)", sum.Created, sum.Updated, sum.Skipped)
	}

	// Change a name → one update.
	recs[0]["name"] = "Party One Renamed"
	sum = runImport(t, pool, ctx, handler, recs, prov)
	if sum.Updated != 1 {
		t.Fatalf("renamed run: updated=%d (want 1)", sum.Updated)
	}

	var name, source string
	if err := pool.QueryRow(ctx,
		`SELECT name, source FROM oikumenea.external_organizations WHERE wikidata_id = $1 AND deleted_at IS NULL`, q1).Scan(&name, &source); err != nil {
		t.Fatalf("verify imported row: %v", err)
	}
	if name != "Party One Renamed" || source != domain.SourceImported {
		t.Fatalf("imported row name=%q source=%q", name, source)
	}
}

func runImport(t *testing.T, pool *pgxpool.Pool, ctx context.Context, h dataimportapp.Handler, recs []dataimportdomain.Record, prov dataimportdomain.Provenance) dataimportdomain.Summary {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	sum, err := h(ctx, tx, recs, prov, dataimportdomain.ChunkInfo{Seq: 1, IsLast: true})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return sum
}
