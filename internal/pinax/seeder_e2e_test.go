//go:build integration

// End-to-end verification of the pinax reference-plane autoseeder (D-Pinax, M45) against a real
// Postgres with the migrations applied. It drives the SAME Seeder the boot autoseed hook and the
// `oikumenea seed` CLI use — the bundled go:embed presets self-seeded in-process, with NO hermenea /
// network dependency (so "fresh boot with hermenea down" is inherent). It asserts the load-bearing
// invariants: create-if-absent, the version-gated re-boot no-op, seeded rows marked origin='seeded',
// and an operator row (origin='operator') surviving a --reconcile.
//
// Run against a throwaway DB that has the migrations applied (PostGIS + h3 required):
//
//	OIKUMENEA_PINAX_E2E=1 OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestPinaxSeeder -timeout 300s ./internal/pinax/...
package pinax_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/dataimport"
	importdomain "github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/pinax"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	rankadapters "github.com/olegamysk/go-oikumenea/internal/rank/adapters"
	rankapp "github.com/olegamysk/go-oikumenea/internal/rank/application"
	rankdomain "github.com/olegamysk/go-oikumenea/internal/rank/domain"
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

// newSeeder wires the Seeder exactly as the composition root does (embedded presets only).
func newSeeder(t *testing.T, pool *pgxpool.Pool) *pinax.Seeder {
	return newSeederWithPacks(t, pool, "")
}

// newSeederWithPacks wires the Seeder as the composition root does — the flat-handler import service plus
// the `ranks` native importer (rank.Service.ImportPreset over its own tx) — additionally scanning an
// operator-mounted packs directory (D-DataPacks, M54; "" = embedded-only).
func newSeederWithPacks(t *testing.T, pool *pgxpool.Pool, packsDir string) *pinax.Seeder {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	rankSvc := rankapp.NewService(pool,
		func(conn pdb.DBTX) rankdomain.Repository { return rankadapters.NewRepository(conn) }, audit)
	native := map[string]pinax.NativeImporter{
		"ranks": func(ctx context.Context, records []map[string]any, _ bool) (importdomain.Summary, error) {
			var sum importdomain.Summary
			for _, rec := range records {
				p, err := rankapp.PresetFromMap(rec)
				if err != nil {
					return importdomain.Summary{}, err
				}
				s, err := rankSvc.ImportPreset(ctx, p)
				if err != nil {
					return importdomain.Summary{}, err
				}
				sum.Created += s.Created
				sum.Updated += s.Updated
				sum.Skipped += s.Skipped
			}
			return sum, nil
		},
	}
	seeder, err := pinax.NewSeeder(pool, dataimport.NewImportService(pool, audit), native, packsDir)
	if err != nil {
		t.Fatalf("new seeder: %v", err)
	}
	return seeder
}

func TestPinaxSeeder(t *testing.T) {
	if os.Getenv("OIKUMENEA_PINAX_E2E") == "" {
		t.Skip("set OIKUMENEA_PINAX_E2E=1 to run the pinax seeder e2e (loads the full presets)")
	}
	ctx := context.Background()
	pool := newPool(t)
	seeder := newSeeder(t, pool)

	// Reset the `religions` preset to a fresh state so this run must actually create: drop the version
	// marker and the one preset-created leaf taxon (ahmadiyya has no children / color_id, so it deletes
	// cleanly). The migration-seeded curated tree stays put — the seeder must skip it (create-if-absent).
	resetReligions(t, pool)

	// 1) First seed (boot autoseed, create-if-absent). Runs every preset in dependency order; on a warm
	//    DB the others no-op, but religions (just reset) must re-apply and create ahmadiyya.
	first, err := seeder.Seed(ctx, false)
	if err != nil {
		t.Fatalf("seed(create-if-absent): %v", err)
	}
	rel, ok := first["religions"]
	if !ok || rel.Created < 1 {
		t.Fatalf("religions preset should have created ≥1 taxon, got %+v (ran: %v)", rel, keys(first))
	}
	// ahmadiyya exists, is seeded-owned, and its denormalized root religion_id was derived (closure ran).
	assertReligion(t, pool, "ahmadiyya", "seeded", true)

	// 2) Second seed (create-if-absent) is a version-gated no-op — the re-boot case. Nothing re-applies.
	second, err := seeder.Seed(ctx, false)
	if err != nil {
		t.Fatalf("seed(re-boot): %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second seed must be a version-gated no-op, ran %v", keys(second))
	}

	// 3) An operator-owned taxon (origin='operator') must SURVIVE a --reconcile (which re-runs every
	//    preset with update-on-change). Reconcile only touches seeded rows.
	seedOperatorReligion(t, pool)
	if _, err := seeder.Seed(ctx, true); err != nil {
		t.Fatalf("seed(--reconcile): %v", err)
	}
	assertReligion(t, pool, "pinax_e2e_operator_faith", "operator", false)

	// cleanup
	if _, err := pool.Exec(ctx, "DELETE FROM oikumenea.religion_taxa WHERE code='pinax_e2e_operator_faith'"); err != nil {
		t.Fatalf("cleanup operator taxon: %v", err)
	}
}

func resetReligions(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`DELETE FROM oikumenea.religion_taxon_classifications tc USING oikumenea.religion_taxa t
		   WHERE t.id = tc.taxon_id AND t.code = 'ahmadiyya'`,
		`DELETE FROM oikumenea.religion_taxa_closure c USING oikumenea.religion_taxa t
		   WHERE (c.ancestor_id = t.id OR c.descendant_id = t.id) AND t.code = 'ahmadiyya'`,
		`DELETE FROM oikumenea.religion_taxa WHERE code = 'ahmadiyya'`,
		`DELETE FROM oikumenea.pinax_seed_state WHERE preset = 'religions'`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("reset religions (%s): %v", s, err)
		}
	}
}

func seedOperatorReligion(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO oikumenea.religion_taxa (code, name, rank_id, parent_id, origin)
		SELECT 'pinax_e2e_operator_faith', 'Operator E2E Faith', r.id, NULL, 'operator'
		FROM oikumenea.religion_taxon_ranks r WHERE r.code = 'religion'
		ON CONFLICT DO NOTHING`)
	if err != nil {
		t.Fatalf("seed operator taxon: %v", err)
	}
}

func assertReligion(t *testing.T, pool *pgxpool.Pool, code, wantOrigin string, wantRoot bool) {
	t.Helper()
	var origin string
	var hasRoot bool
	err := pool.QueryRow(context.Background(),
		"SELECT origin, religion_id IS NOT NULL FROM oikumenea.religion_taxa WHERE code=$1 AND deleted_at IS NULL",
		code).Scan(&origin, &hasRoot)
	if err != nil {
		t.Fatalf("read taxon %q: %v", code, err)
	}
	if origin != wantOrigin {
		t.Fatalf("taxon %q origin = %q, want %q", code, origin, wantOrigin)
	}
	if wantRoot && !hasRoot {
		t.Fatalf("taxon %q religion_id not derived (closure did not run)", code)
	}
}

func keys(m map[string]importdomain.Summary) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
