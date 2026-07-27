// seed-demo: realistic "used system" demo data generator.
//
// Fills a MIGRATED oikumenea database with a believable, fully-interrelated world so the console and
// APIs look like a system in real use: several unit trees (military brigades + a university, plus
// small company/church satellites), 100 persons appointed to positions (military persons carrying
// the corresponding UA-armed-forces rank for their echelon), 200 more persons woven into family
// relationships (heterosexual marriages, children, parents, siblings, guardianships, …), and a
// spread of every module's tables — contacts, documents, addresses, vehicles, education, company,
// religion, overlays, watchlists — INCLUDING the envelope-encrypted ones (finance, health, legal,
// political leaning, party, ethnicity), sealed with the app's own crypto provider.
//
//	go run ./scripts/seed-demo -dsn "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable"
//	go run ./scripts/seed-demo -dsn ... -reset   # delete previously seeded demo rows first
//
// PREREQUISITES: the DB must be migrated (atlas migrate apply --env local) AND the pinax autoseeder
// must have run (boot the server once, or `oikumenea seed`) — it loads the rank scheme, ethnicity
// types and Glottolog languoids, none of which are migration-seeded. The seeder verifies this and
// exits with guidance if missing.
//
// All demo top-level rows (organizations, persons) are tagged {"seed":"demo"} in metadata/attributes
// so -reset can find and remove exactly the demo data. Deterministic under -seed.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"time"

	"github.com/olegamysk/go-oikumenea/pkg/crypto"

	"github.com/jackc/pgx/v5"
)

func main() {
	dsn := flag.String("dsn", "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable", "target Postgres DSN (migrations must be applied)")
	configPath := flag.String("config", "var/conf/install.yml", "path to install.yml (for the crypto keys)")
	seed := flag.Int64("seed", 42, "PRNG seed (world is deterministic per seed)")
	reset := flag.Bool("reset", false, "delete previously seeded demo rows before seeding")
	flag.Parse()

	if err := run(context.Background(), *dsn, *configPath, *seed, *reset); err != nil {
		fmt.Fprintln(os.Stderr, "seed-demo:", err)
		os.Exit(1)
	}
}

// world holds the ids threaded across phases.
type person struct {
	id  string
	sex string // "male" | "female"
}

type seeder struct {
	ctx    context.Context
	tx     pgx.Tx
	rng    *rand.Rand
	cipher *crypto.Cipher

	// resolved reference ids
	uaSystemID string
	countryUA  string
	countryUS  string
	langUA     string // a languoid id (Ukrainian if resolvable, else any)

	// people
	appointed []person // 100 on positions
	relatives []person // 200 family members
	counts    map[string]int
}

func run(ctx context.Context, dsn, configPath string, seed int64, reset bool) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	cipher, err := buildCipher(configPath)
	if err != nil {
		return fmt.Errorf("build crypto cipher (need %s with dev keys): %w", configPath, err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rolls back only if Commit didn't run

	s := &seeder{ctx: ctx, tx: tx, rng: rand.New(rand.NewSource(seed)), cipher: cipher, counts: map[string]int{}}

	if reset {
		if err := s.reset(); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
	}
	if err := s.checkPrereqs(); err != nil {
		return err
	}
	if !reset {
		var existing int
		_ = s.scalar(&existing, `SELECT count(*) FROM oikumenea.tenant_organizations WHERE metadata->>'seed'='demo'`)
		if existing > 0 {
			return fmt.Errorf("demo data already present (%d demo orgs) — re-run with -reset to replace it", existing)
		}
	}

	start := time.Now()
	if err := s.phaseADirectory(); err != nil {
		return fmt.Errorf("phase A (directory): %w", err)
	}
	if err := s.phaseBPersons(); err != nil {
		return fmt.Errorf("phase B (persons/ranks/memberships): %w", err)
	}
	if err := s.phaseCRelationships(); err != nil {
		return fmt.Errorf("phase C (relationships): %w", err)
	}
	if err := s.phaseDEnrichment(); err != nil {
		return fmt.Errorf("phase D (enrichment): %w", err)
	}
	if err := s.phaseEEncrypted(); err != nil {
		return fmt.Errorf("phase E (encrypted): %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	s.printSummary(time.Since(start))
	return nil
}

// ---- helpers ---------------------------------------------------------------------------------

// ins runs an INSERT ... RETURNING id and returns the minted RID (letting the column DEFAULT
// new_id() fire — the shape-valid pattern the integration fixtures use). Also bumps a named counter.
func (s *seeder) ins(counter, sql string, args ...any) (string, error) {
	var id string
	if err := s.tx.QueryRow(s.ctx, sql, args...).Scan(&id); err != nil {
		return "", fmt.Errorf("%s: %w", counter, err)
	}
	s.counts[counter]++
	return id, nil
}

// exec runs a statement with no returned id and bumps a counter by rows affected.
func (s *seeder) exec(counter, sql string, args ...any) error {
	ct, err := s.tx.Exec(s.ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", counter, err)
	}
	s.counts[counter] += int(ct.RowsAffected())
	return nil
}

func (s *seeder) scalar(dest any, sql string, args ...any) error {
	return s.tx.QueryRow(s.ctx, sql, args...).Scan(dest)
}

func (s *seeder) chance(p float64) bool { return s.rng.Float64() < p }
func (s *seeder) pick(xs []string) string {
	return xs[s.rng.Intn(len(xs))]
}

func (s *seeder) checkPrereqs() error {
	if err := s.scalar(&s.uaSystemID, `SELECT id::text FROM oikumenea.rank_systems WHERE code='ua-armed-forces'`); err != nil {
		return fmt.Errorf("ua-armed-forces rank system not found — run `oikumenea seed` (or boot the server) to run the pinax autoseeder first: %w", err)
	}
	var n int
	if err := s.scalar(&n, `SELECT count(*) FROM oikumenea.person_ethnicity_types`); err != nil || n == 0 {
		return fmt.Errorf("person_ethnicity_types is empty — run the pinax autoseeder (`oikumenea seed`) first")
	}
	if err := s.scalar(&n, `SELECT count(*) FROM oikumenea.language_languoids`); err != nil || n == 0 {
		return fmt.Errorf("language_languoids is empty — run the pinax autoseeder (`oikumenea seed`) first")
	}
	if err := s.scalar(&s.countryUA, `SELECT id::text FROM oikumenea.geo_countries WHERE code='UA'`); err != nil {
		return fmt.Errorf("country UA not found: %w", err)
	}
	_ = s.scalar(&s.countryUS, `SELECT id::text FROM oikumenea.geo_countries WHERE code='US'`)
	if err := s.scalar(&s.langUA, `SELECT id::text FROM oikumenea.language_languoids ORDER BY (name='Ukrainian') DESC, id LIMIT 1`); err != nil {
		return fmt.Errorf("no languoid found: %w", err)
	}
	return nil
}

// reset removes previously-seeded demo rows. Persons cascade to their person_* children,
// memberships and relationships; then positions/units/graphs/orgs and satellite catalog rows.
func (s *seeder) reset() error {
	demoOrgs := `SELECT id FROM oikumenea.tenant_organizations WHERE metadata->>'seed'='demo'`
	demoUnits := `SELECT id FROM oikumenea.tenant_units WHERE org_id IN (` + demoOrgs + `)`
	stmts := []string{
		// Rows that DON'T cascade from person/org and would otherwise block, first.
		`DELETE FROM oikumenea.company_appointments a USING oikumenea.person_persons p WHERE a.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.education_appointments a USING oikumenea.person_persons p WHERE a.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.membership_memberships m USING oikumenea.person_persons p WHERE m.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.vehicle_registrations r USING oikumenea.vehicle_vehicles v WHERE r.vehicle_id=v.id AND v.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.vehicle_vehicles WHERE attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.vehicle_models WHERE code LIKE 'demo-%'`,
		`DELETE FROM oikumenea.vehicle_brands WHERE code LIKE 'demo-%'`,
		`DELETE FROM oikumenea.finance_accounts WHERE institution_id IN (` + demoOrgs + `)`, // cards cascade
		// person-linked tables with a non-CASCADE FK to person must be cleared before persons.
		`DELETE FROM oikumenea.document_documents x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.document_personal_codes x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.person_ethnicities x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.person_health_records x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.person_legal_records x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.person_party_memberships x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.person_political_leaning x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.religion_affiliations x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.religion_clergy_credentials x USING oikumenea.person_persons p WHERE x.person_id=p.id AND p.attributes->>'seed'='demo'`,
		`DELETE FROM oikumenea.person_persons WHERE attributes->>'seed'='demo'`, // cascades remaining person_* children
		`DELETE FROM oikumenea.person_dormitory_stays s USING oikumenea.education_buildings b WHERE s.building_id=b.id AND b.code LIKE 'demo-%'`,
		`DELETE FROM oikumenea.education_buildings WHERE code LIKE 'demo-%'`,
		`DELETE FROM oikumenea.location_locations WHERE raw_address='DEMO'`,
		`DELETE FROM oikumenea.education_qualifications WHERE institution_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.education_positions WHERE institution_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.education_groups WHERE unit_id IN (` + demoUnits + `)`,
		`DELETE FROM oikumenea.education_programs WHERE institution_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.company_positions WHERE company_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.external_organizations WHERE name LIKE 'DEMO %'`,
		`DELETE FROM oikumenea.membership_positions WHERE unit_id IN (` + demoUnits + `)`,
		`DELETE FROM oikumenea.education_org_profiles WHERE institution_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.company_org_profiles WHERE company_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.religion_org_profiles WHERE unit_id IN (` + demoUnits + `)`,
		`DELETE FROM oikumenea.tenant_unit_closure c USING oikumenea.tenant_graphs g WHERE c.graph_id=g.id AND g.org_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.tenant_unit_edges e USING oikumenea.tenant_graphs g WHERE e.graph_id=g.id AND g.org_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.tenant_units WHERE org_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.tenant_graphs WHERE org_id IN (` + demoOrgs + `)`,
		`DELETE FROM oikumenea.tenant_organizations WHERE metadata->>'seed'='demo'`,
	}
	for _, q := range stmts {
		if _, err := s.tx.Exec(s.ctx, q); err != nil {
			return fmt.Errorf("reset step %q: %w", q[:min(48, len(q))], err)
		}
	}
	fmt.Println("==> reset: previously-seeded demo rows deleted")
	return nil
}

func (s *seeder) printSummary(d time.Duration) {
	fmt.Printf("==> seeded in %s\n", d.Round(time.Millisecond))
	order := []string{"org", "graph", "unit", "edge", "position", "person", "rank", "membership",
		"marriage", "kinship", "guardianship", "next_of_kin", "sponsorship", "association",
		"email", "phone", "call_sign", "messenger", "social", "document", "name_variant",
		"citizenship", "residence", "language", "location", "address", "dormitory", "vehicle",
		"registration", "enrollment", "qualification", "edu_appointment", "company_appointment",
		"beneficiary", "shareholding", "company_reg", "affiliation", "clergy", "external_org",
		"physical", "mark", "personality", "gov_position", "insurance", "sanction", "wallet", "watchlist",
		"finance_account", "finance_card", "health", "legal", "political", "party", "ethnicity"}
	for _, k := range order {
		if s.counts[k] > 0 {
			fmt.Printf("    %-18s %d\n", k, s.counts[k])
		}
	}
}

// buildCipher mirrors cmd/oikumenea/main.go buildCipher / rewrap.go: it reads the two base64 dev keys
// from install.yml and constructs the same local-dev cipher the server uses, so sealed ciphertext
// decrypts in-app.
func buildCipher(configPath string) (*crypto.Cipher, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	kek, err := extractB64(raw, `(?m)^\s*kek:\s*"([^"]+)"`)
	if err != nil {
		return nil, fmt.Errorf("crypto.local-dev.kek: %w", err)
	}
	blind, err := extractB64(raw, `(?m)blind-index-key:\s*"([^"]+)"`)
	if err != nil {
		return nil, fmt.Errorf("crypto.blind-index-key: %w", err)
	}
	kp, err := crypto.NewLocalDevProvider(kek)
	if err != nil {
		return nil, err
	}
	return crypto.NewCipher(kp, blind, 5*time.Minute)
}

func extractB64(raw []byte, pattern string) ([]byte, error) {
	m := regexp.MustCompile(pattern).FindSubmatch(raw)
	if m == nil {
		return nil, fmt.Errorf("key not found in config")
	}
	return base64.StdEncoding.DecodeString(string(m[1]))
}

// seal encrypts a value with the app cipher and returns the four envelope column values.
func (s *seeder) seal(v string) (ciphertext, wrappedDEK []byte, keyRef string, blind []byte, err error) {
	sealed, err := s.cipher.Seal(s.ctx, []byte(v))
	if err != nil {
		return nil, nil, "", nil, err
	}
	return sealed.Ciphertext, sealed.WrappedDEK, sealed.KeyRef, s.cipher.BlindIndex([]byte(v)), nil
}
