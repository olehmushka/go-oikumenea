// seed-scale: synthetic national-scale world generator (review-2026-07 Phase 0 / M46).
//
// Seeds ONE authority-bearing graph with -units tenant units (multi-parent DAG), -persons persons
// with active memberships, and a realistic grant mix, so the authorization-path fixes (review
// R-01..R-04) can be measured instead of guessed. Deterministic under -seed.
//
//	go run ./scripts/seed-scale -dsn "postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable"
//
// The target database must have migrations applied (atlas migrate apply --env local) and must NOT
// be the operator dev DB `postgres` (refused, same rule as scripts/setup-test-db.sh). Re-running
// against an already-seeded DB refuses and just re-prints the probe subjects.
//
// Probe subjects (person.code → what they measure):
//	scale-root-subject  subtree grant at the graph root  → reach ≈ the whole org (the R-02 wall)
//	scale-mid-subject   subtree grant mid-tree           → reach ≈ 10²–10⁴ units
//	scale-leaf-subject  unit grant on one leaf           → minimal reach
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	domainCode = "scale-domain"
	orgCode    = "scale-org"
	graphCode  = "scale-command"

	rootSubjectCode = "scale-root-subject"
	midSubjectCode  = "scale-mid-subject"
	leafSubjectCode = "scale-leaf-subject"
)

func main() {
	dsn := flag.String("dsn", "", "target Postgres DSN (required; migrations must be applied; never the dev 'postgres' DB)")
	units := flag.Int("units", 100_000, "tenant units in the graph")
	persons := flag.Int("persons", 1_000_000, "persons in the directory")
	randomSubjects := flag.Int("random-subjects", 1_000, "additional random grant-holding subjects")
	seed := flag.Int64("seed", 1, "PRNG seed (world is deterministic per seed)")
	flag.Parse()

	if err := run(context.Background(), *dsn, *units, *persons, *randomSubjects, *seed); err != nil {
		fmt.Fprintln(os.Stderr, "seed-scale:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dsn string, nUnits, nPersons, nRandomSubjects int, seed int64) error {
	if dsn == "" {
		return fmt.Errorf("-dsn is required")
	}
	dbName := dsn[strings.LastIndex(dsn, "/")+1:]
	if i := strings.IndexByte(dbName, '?'); i >= 0 {
		dbName = dbName[:i]
	}
	if dbName == "" || dbName == "postgres" {
		return fmt.Errorf("refusing to seed %q — use a dedicated database (e.g. oikumenea_scale)", dbName)
	}
	if nUnits < 2 || nPersons < 1 {
		return fmt.Errorf("need at least 2 units and 1 person")
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	// Best-effort: only works when the server preloads the library (compose files do).
	if _, err := conn.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_stat_statements"); err != nil {
		fmt.Println("note: pg_stat_statements unavailable (no shared_preload_libraries); continuing")
	}

	var seeded bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM oikumenea.tenant_organizations WHERE code = $1)", orgCode,
	).Scan(&seeded); err != nil {
		return fmt.Errorf("check existing world (are migrations applied?): %w", err)
	}
	if seeded {
		fmt.Println("world already seeded — refusing to re-seed. Probe subjects:")
		return printProbeSubjects(ctx, conn)
	}

	rng := rand.New(rand.NewSource(seed))
	mint := newMinter(rng)
	start := time.Now()

	// --- Structural scaffolding: domain → org → graph -----------------------------------------
	domainID, orgID, graphID := mint.rid(4, 1, 5), mint.rid(4, 1, 6), mint.rid(4, 1, 2)
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.tenant_domains (id, code, name, pdp_scoped) VALUES ($1, $2, 'Scale harness domain', true)`,
		domainID, domainCode); err != nil {
		return fmt.Errorf("seed domain: %w", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.tenant_organizations (id, code, name, domain_id) VALUES ($1, $2, 'Scale harness org', $3)`,
		orgID, orgCode, domainID); err != nil {
		return fmt.Errorf("seed org: %w", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.tenant_graphs (id, org_id, code, name, is_default, is_authority_bearing)
		VALUES ($1, $2, $3, 'Scale harness command graph', true, true)`,
		graphID, orgID, graphCode); err != nil {
		return fmt.Errorf("seed graph: %w", err)
	}

	// --- Units: random recursive tree (avg depth ≈ ln n) + ~1% extra edges → multi-parent DAG --
	unitIDs := make([][16]byte, nUnits)
	for i := range unitIDs {
		unitIDs[i] = mint.rid(4, 1, 1)
	}
	fmt.Printf("==> copying %d tenant_units\n", nUnits)
	if _, err := conn.CopyFrom(ctx,
		pgx.Identifier{"oikumenea", "tenant_units"},
		[]string{"id", "org_id", "domain_id", "name"},
		pgx.CopyFromSlice(nUnits, func(i int) ([]any, error) {
			return []any{unitIDs[i], orgID, domainID, fmt.Sprintf("Scale unit %d", i)}, nil
		})); err != nil {
		return fmt.Errorf("copy units: %w", err)
	}

	// Random recursive tree (parent uniform among earlier units → acyclic, avg depth ≈ ln n) plus
	// ~1% deduplicated second-parent edges so the graph is a genuine multi-parent DAG. The dedupe
	// matters: tenant_unit_edges has UNIQUE (graph_id, parent_id, child_id) and COPY aborts on
	// conflict.
	type edge struct{ parent, child int }
	seen := make(map[edge]struct{}, nUnits+nUnits/100)
	edges := make([]edge, 0, nUnits+nUnits/100)
	addEdge := func(e edge) {
		if _, dup := seen[e]; dup {
			return
		}
		seen[e] = struct{}{}
		edges = append(edges, e)
	}
	for i := 1; i < nUnits; i++ {
		addEdge(edge{parent: rng.Intn(i), child: i})
	}
	for i := 0; i < nUnits/100; i++ {
		child := 1 + rng.Intn(nUnits-1)
		addEdge(edge{parent: rng.Intn(child), child: child})
	}
	fmt.Printf("==> copying %d tenant_unit_edges\n", len(edges))
	if _, err := conn.CopyFrom(ctx,
		pgx.Identifier{"oikumenea", "tenant_unit_edges"},
		[]string{"id", "graph_id", "parent_id", "child_id"},
		pgx.CopyFromSlice(len(edges), func(i int) ([]any, error) {
			return []any{mint.rid(4, 2, 1), graphID, unitIDs[edges[i].parent], unitIDs[edges[i].child]}, nil
		})); err != nil {
		return fmt.Errorf("copy edges: %w", err)
	}

	fmt.Println("==> rebuilding closure (one recursive CTE pass — the one-time seed path, not R-04)")
	closureStart := time.Now()
	if _, err := conn.Exec(ctx, `
		WITH RECURSIVE
		  nodes AS (
		    SELECT parent_id AS u FROM oikumenea.tenant_unit_edges WHERE graph_id = $1
		    UNION
		    SELECT child_id FROM oikumenea.tenant_unit_edges WHERE graph_id = $1
		  ),
		  reach AS (
		    SELECT u AS ancestor_id, u AS descendant_id, 0 AS depth FROM nodes
		    UNION ALL
		    SELECT r.ancestor_id, e.child_id, r.depth + 1
		    FROM reach r
		    JOIN oikumenea.tenant_unit_edges e
		      ON e.graph_id = $1 AND e.parent_id = r.descendant_id
		  )
		INSERT INTO oikumenea.tenant_unit_closure (graph_id, ancestor_id, descendant_id, depth)
		SELECT $1::uuid, ancestor_id, descendant_id, min(depth)::int
		FROM reach
		GROUP BY ancestor_id, descendant_id`, graphID); err != nil {
		return fmt.Errorf("rebuild closure: %w", err)
	}
	var closureRows int64
	_ = conn.QueryRow(ctx, "SELECT count(*) FROM oikumenea.tenant_unit_closure WHERE graph_id = $1", graphID).Scan(&closureRows)
	fmt.Printf("    closure: %d rows in %s\n", closureRows, time.Since(closureStart).Round(time.Second))

	// --- Persons + memberships ------------------------------------------------------------------
	surnames := makeSurnames(rng)
	personIDs := make([][16]byte, nPersons)
	for i := range personIDs {
		personIDs[i] = mint.rid(6, 1, 1)
	}
	fmt.Printf("==> copying %d person_persons\n", nPersons)
	if _, err := conn.CopyFrom(ctx,
		pgx.Identifier{"oikumenea", "person_persons"},
		[]string{"id", "display_name", "given", "surname"},
		pgx.CopyFromSlice(nPersons, func(i int) ([]any, error) {
			given := fmt.Sprintf("Given%d", i%9973)
			surname := surnames[i%len(surnames)]
			return []any{personIDs[i], given + " " + surname, given, surname}, nil
		})); err != nil {
		return fmt.Errorf("copy persons: %w", err)
	}
	fmt.Printf("==> copying %d membership_memberships\n", nPersons)
	if _, err := conn.CopyFrom(ctx,
		pgx.Identifier{"oikumenea", "membership_memberships"},
		[]string{"id", "person_id", "unit_id"},
		pgx.CopyFromSlice(nPersons, func(i int) ([]any, error) {
			return []any{mint.rid(7, 2, 1), personIDs[i], unitIDs[rng.Intn(nUnits)]}, nil
		})); err != nil {
		return fmt.Errorf("copy memberships: %w", err)
	}

	// --- Grant mix -------------------------------------------------------------------------------
	// One role carrying a read + a write permission (the PDP classifies by the `.read` suffix).
	roleID := mint.rid(8, 1, 1)
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.authz_roles (id, code, name) VALUES ($1, 'scale-reader', 'Scale harness reader')`,
		roleID); err != nil {
		return fmt.Errorf("seed role: %w", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code)
		VALUES ($1, 'person.read'), ($1, 'unit.read'), ($1, 'person.update')`, roleID); err != nil {
		return fmt.Errorf("seed role permissions: %w", err)
	}

	// Probe subjects: extra persons with stable codes so re-runs / measurement scripts can find them.
	probe := func(code string) ([16]byte, error) {
		id := mint.rid(6, 1, 1)
		_, err := conn.Exec(ctx, `
			INSERT INTO oikumenea.person_persons (id, code, display_name, given, surname)
			VALUES ($1, $2, $2, 'Probe', 'Subject')`, id, code)
		return id, err
	}
	rootSubject, err := probe(rootSubjectCode)
	if err != nil {
		return fmt.Errorf("seed probe subject: %w", err)
	}
	midSubject, err := probe(midSubjectCode)
	if err != nil {
		return fmt.Errorf("seed probe subject: %w", err)
	}
	leafSubject, err := probe(leafSubjectCode)
	if err != nil {
		return fmt.Errorf("seed probe subject: %w", err)
	}

	// Root = unit 0 (every other unit descends from it by construction).
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
		VALUES ($1, $2, $3, 'subtree', $4)`, rootSubject, roleID, unitIDs[0], graphID); err != nil {
		return fmt.Errorf("grant root subject: %w", err)
	}
	// Mid-tree: a unit whose subtree is 200..20000 units (picked from the freshly built closure).
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
		SELECT $1, $2, c.ancestor_id, 'subtree', $3
		FROM oikumenea.tenant_unit_closure c
		WHERE c.graph_id = $3
		GROUP BY c.ancestor_id
		HAVING count(*) BETWEEN 200 AND 20000
		ORDER BY c.ancestor_id
		LIMIT 1`, midSubject, roleID, graphID); err != nil {
		return fmt.Errorf("grant mid subject: %w", err)
	}
	// Leaf: unit-scope grant on a leaf (no descendants beyond itself).
	if _, err := conn.Exec(ctx, `
		INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
		SELECT $1, $2, c.ancestor_id, 'unit'
		FROM oikumenea.tenant_unit_closure c
		WHERE c.graph_id = $3
		GROUP BY c.ancestor_id
		HAVING count(*) = 1
		ORDER BY c.ancestor_id
		LIMIT 1`, leafSubject, roleID, graphID); err != nil {
		return fmt.Errorf("grant leaf subject: %w", err)
	}
	// Random grant holders: unit or subtree, targets uniform over units.
	fmt.Printf("==> granting %d random subjects\n", nRandomSubjects)
	for i := 0; i < nRandomSubjects; i++ {
		subj := personIDs[rng.Intn(nPersons)]
		target := unitIDs[rng.Intn(nUnits)]
		if rng.Intn(2) == 0 {
			_, err = conn.Exec(ctx, `
				INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
				VALUES ($1, $2, $3, 'unit') ON CONFLICT DO NOTHING`, subj, roleID, target)
		} else {
			_, err = conn.Exec(ctx, `
				INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
				VALUES ($1, $2, $3, 'subtree', $4) ON CONFLICT DO NOTHING`, subj, roleID, target, graphID)
		}
		if err != nil {
			return fmt.Errorf("grant random subject: %w", err)
		}
	}

	fmt.Printf("==> world seeded in %s\n", time.Since(start).Round(time.Second))
	return printProbeSubjects(ctx, conn)
}

func printProbeSubjects(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, `
		SELECT p.code, p.id::text,
		       coalesce((SELECT count(*) FROM oikumenea.authz_role_assignments a
		                 LEFT JOIN oikumenea.tenant_unit_closure c
		                   ON a.scope = 'subtree' AND c.graph_id = a.graph_id AND c.ancestor_id = a.target_unit_id
		                 WHERE a.subject_person_id = p.id AND a.revoked_at IS NULL), 0) AS reach_units
		FROM oikumenea.person_persons p
		WHERE p.code IN ($1, $2, $3)
		ORDER BY p.code`, rootSubjectCode, midSubjectCode, leafSubjectCode)
	if err != nil {
		return fmt.Errorf("list probe subjects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var code, id string
		var reach int64
		if err := rows.Scan(&code, &id, &reach); err != nil {
			return err
		}
		fmt.Printf("    %-22s %s  (~%d reach rows)\n", code, id, reach)
	}
	return rows.Err()
}

// minter packs RIDs (native UUIDv8, D-ResourceIdentifiers / F-014) client-side, mirroring
// oikumenea.new_id(): bytes 0..5 unix-ms, byte6 version|kind, byte7 app=1, byte8 variant|service,
// byte9..10 type + random, bytes 11..15 random. The ms field is a monotonic counter here (one
// tick per 256 IDs) so bulk minting cannot birthday-collide on the ~44 random bits.
type minter struct {
	rng  *rand.Rand
	base uint64
	n    uint64
}

func newMinter(rng *rand.Rand) *minter {
	return &minter{rng: rng, base: uint64(time.Now().UnixMilli())}
}

func (m *minter) rid(service, kind, typ int) [16]byte {
	var b [16]byte
	m.rng.Read(b[10:])
	ms := m.base + m.n/256
	m.n++
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], ms)
	copy(b[0:6], ts[2:8])
	b[6] = 0x80 | byte(kind&0x0f)
	b[7] = 1
	b[8] = 0x80 | byte(service&0x3f)
	b[9] = byte(typ & 0xff)
	b[10] = byte(((typ>>8)&0x0f)<<4) | (b[10] & 0x0f)
	return b
}

func makeSurnames(rng *rand.Rand) []string {
	out := make([]string, 200)
	syllables := []string{"ko", "shen", "chuk", "enko", "ov", "iv", "ak", "yk", "ets", "un"}
	for i := range out {
		out[i] = fmt.Sprintf("Surname%s%d", syllables[rng.Intn(len(syllables))], i)
	}
	return out
}
