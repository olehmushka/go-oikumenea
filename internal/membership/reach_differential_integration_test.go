//go:build integration

// Randomized differential test for the SQL reach semi-joins (review-2026-07 R-02.1 parity risk
// control). The R-02 refactor re-implements the reach algebra three times — the Go reference
// (authorization/domain ReachSet, property-tested), membership's semi-join queries, and (R-02.2)
// the RLS policy predicate. This test seeds ~25 random real-DB worlds (multi-graph unit DAGs,
// persons, memberships incl. ended/deleted, roles incl. soft-deleted, assignments incl.
// revoked/expired/directory-only) and asserts, for each world's subject, that the SQL results
// EXACTLY equal the Go oracle:
//
//	VisiblePersonIDsForSubject  ≡ persons with an active membership in ReachSet(grants).Readable
//	SubjectCanReadPerson        ≡ per-person membership of that same set
//	ReadableUnitsForSubjectAmong ≡ ReachSet(grants).Readable ∩ candidates
//
// The two known parity traps are exercised by construction: a directory-only subtree grant
// contributes NOTHING (not even its target), and a subtree target is included without needing a
// reflexive closure row. Every failure names the reproducing seed (OIKUMENEA_DIFF_SEED re-runs it).
package membership_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzadapters "github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	membershipadapters "github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

// diffPermPool: unit-scoped permission codes sampled onto roles; the '.read' subset is what the
// reach classifies as read-bearing (mirror of the domain property test's pool).
var diffPermPool = []string{"person.read", "unit.read", "membership.read", "person.update", "unit.create"}

type diffWorld struct {
	units    []string
	deleted  map[string]bool // soft-deleted units (the never-narrower caveat for the RLS predicate)
	graphs   []string
	persons  []string
	subject  string
	activeIn map[string][]string // person -> unit ids with an ACTIVE membership
}

func TestReachDifferential(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()

	seed := time.Now().UnixNano()
	if s := os.Getenv("OIKUMENEA_DIFF_SEED"); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			t.Fatalf("bad OIKUMENEA_DIFF_SEED: %v", err)
		}
		seed = v
	}
	t.Logf("seed=%d (re-run with OIKUMENEA_DIFF_SEED=%d)", seed, seed)
	r := rand.New(rand.NewSource(seed))

	// The Go oracle's closure port: the tenant application service over the same pool (shared
	// closure DATA, independent reach ALGEBRA — the algebra is what we are differentially testing).
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	tenantSvc := tenantapp.NewService(pool, func(conn pdb.DBTX) tenantdomain.Repository {
		return tenantadapters.NewRepository(conn)
	}, audit)
	pdp := authzdomain.NewPDP(tenantSvc)
	authzRepo := authzadapters.NewRepository(pool)
	memRepo := membershipadapters.NewRepository(pool)

	orgID := diffEnsureOrg(t, pool)

	for wi := 0; wi < 25; wi++ {
		w := genDiffWorld(t, r, pool, orgID)

		// Oracle: the pre-refactor pipeline — active grants + domain.ReachSet.
		grants, err := authzRepo.ActiveGrantsForSubject(ctx, w.subject)
		if err != nil {
			t.Fatalf("seed=%d world=%d grants: %v", seed, wi, err)
		}
		reach, err := pdp.ReachSet(ctx, grants, false)
		if err != nil {
			t.Fatalf("seed=%d world=%d ReachSet: %v", seed, wi, err)
		}
		wantPersons := map[string]bool{}
		for p, units := range w.activeIn {
			for _, u := range units {
				if _, ok := reach.Readable[u]; ok {
					wantPersons[p] = true
					break
				}
			}
		}

		// (a) Directory union: full drain AND a limit-2 keyset drain must both equal the oracle.
		gotFull, err := memRepo.VisiblePersonIDsForSubject(ctx, w.subject, "", "", 100000)
		if err != nil {
			t.Fatalf("seed=%d world=%d VisiblePersonIDsForSubject: %v", seed, wi, err)
		}
		assertSameSet(t, seed, wi, "VisiblePersonIDsForSubject", gotFull, wantPersons)
		var paged []string
		after := ""
		for {
			batch, err := memRepo.VisiblePersonIDsForSubject(ctx, w.subject, after, "", 2)
			if err != nil {
				t.Fatalf("seed=%d world=%d paged drain: %v", seed, wi, err)
			}
			paged = append(paged, batch...)
			if len(batch) < 2 {
				break
			}
			after = batch[len(batch)-1]
		}
		assertSameSet(t, seed, wi, "VisiblePersonIDsForSubject(paged)", paged, wantPersons)

		// (b) Point probe per world person.
		for _, p := range w.persons {
			got, err := memRepo.SubjectCanReadPerson(ctx, w.subject, p)
			if err != nil {
				t.Fatalf("seed=%d world=%d SubjectCanReadPerson(%s): %v", seed, wi, p, err)
			}
			if got != wantPersons[p] {
				t.Fatalf("seed=%d world=%d SubjectCanReadPerson(%s) = %v, oracle %v", seed, wi, p, got, wantPersons[p])
			}
		}

		// (c) Batch unit probe over every world unit.
		gotUnits, err := authzRepo.ReadableUnitsForSubjectAmong(ctx, w.subject, w.units)
		if err != nil {
			t.Fatalf("seed=%d world=%d ReadableUnitsForSubjectAmong: %v", seed, wi, err)
		}
		wantUnits := map[string]bool{}
		for _, u := range w.units {
			if _, ok := reach.Readable[u]; ok {
				wantUnits[u] = true
			}
		}
		assertSameSet(t, seed, wi, "ReadableUnitsForSubjectAmong", gotUnits, wantUnits)

		// (d) The RLS policy predicate (R-02.2, oikumenea.authz_unit_in_reach): keyed on the
		// app.person_id GUC, so probe on a dedicated connection. CONTRACT: never narrower than the
		// Go oracle; exact except that it may additionally pass a SOFT-DELETED unit reached as a
		// subtree descendant (the function cannot read RLS-guarded tenant_units — documented in
		// migration 0011 / D-RLSLiveReach).
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		if _, err := conn.Exec(ctx, "SELECT set_config('app.person_id', $1, false)", w.subject); err != nil {
			t.Fatalf("set person GUC: %v", err)
		}
		for _, u := range w.units {
			var got bool
			if err := conn.QueryRow(ctx, "SELECT oikumenea.authz_unit_in_reach($1, false)", u).Scan(&got); err != nil {
				t.Fatalf("seed=%d world=%d authz_unit_in_reach(%s): %v", seed, wi, u, err)
			}
			want := wantUnits[u]
			if want && !got {
				t.Fatalf("seed=%d world=%d authz_unit_in_reach(%s) = false, oracle says readable (backstop narrower than PDP!)", seed, wi, u)
			}
			if got && !want && !w.deleted[u] {
				t.Fatalf("seed=%d world=%d authz_unit_in_reach(%s) = true, oracle says unreadable (non-deleted unit — real divergence)", seed, wi, u)
			}
		}
		if _, err := conn.Exec(ctx, "SELECT set_config('app.person_id', '', false)"); err != nil {
			t.Fatalf("reset person GUC: %v", err)
		}
		conn.Release()
	}
}

func assertSameSet(t *testing.T, seed int64, wi int, what string, got []string, want map[string]bool) {
	t.Helper()
	gotSet := map[string]bool{}
	for _, g := range got {
		gotSet[g] = true
	}
	if len(gotSet) != len(want) {
		t.Fatalf("seed=%d world=%d %s: got %d, oracle %d\ngot:  %v\nwant: %v", seed, wi, what, len(gotSet), len(want), sortedKeys(gotSet), sortedKeys(want))
	}
	for k := range want {
		if !gotSet[k] {
			t.Fatalf("seed=%d world=%d %s: oracle expects %s, SQL missing it", seed, wi, what, k)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func diffEnsureOrg(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('diff-domain','Diff Domain')
		  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("ensure domain: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
		  SELECT 'diff-org','Diff Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='diff-domain'
		  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	var id string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM oikumenea.tenant_organizations WHERE code='diff-org' AND deleted_at IS NULL`).Scan(&id); err != nil {
		t.Fatalf("org id: %v", err)
	}
	return id
}

// genDiffWorld materializes one random world as real rows: units (some soft-deleted), 1–3 graphs
// (~1 in 4 directory-only) with sparse low→high DAG edges + a rebuilt closure, persons with
// active/ended/deleted memberships, 1–2 roles (random permission sets; occasionally soft-deleted),
// and 0–6 assignments for a fresh subject (some revoked, some expired).
func genDiffWorld(t *testing.T, r *rand.Rand, pool *pgxpool.Pool, orgID string) diffWorld {
	t.Helper()
	ctx := context.Background()
	w := diffWorld{activeIn: map[string][]string{}}
	tag := uuid.NewString()[:8]

	n := 3 + r.Intn(10)
	for i := 0; i < n; i++ {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO oikumenea.tenant_units (org_id, domain_id, name)
			SELECT $1, o.domain_id, $2 FROM oikumenea.tenant_organizations o WHERE o.id = $1
			RETURNING id`, orgID, fmt.Sprintf("diff-%s-u%02d", tag, i)).Scan(&id); err != nil {
			t.Fatalf("seed unit: %v", err)
		}
		w.units = append(w.units, id)
	}
	// Soft-delete ~1 in 8 units (parity trap: a deleted unit is excluded from subtree descendants
	// but still reachable as a direct target).
	w.deleted = map[string]bool{}
	for _, u := range w.units {
		if r.Intn(8) == 0 {
			if _, err := pool.Exec(ctx, `UPDATE oikumenea.tenant_units SET deleted_at = now() WHERE id = $1`, u); err != nil {
				t.Fatalf("soft-delete unit: %v", err)
			}
			w.deleted[u] = true
		}
	}

	nGraphs := 1 + r.Intn(3)
	for gi := 0; gi < nGraphs; gi++ {
		var gid string
		bearing := r.Intn(4) != 0
		if err := pool.QueryRow(ctx, `
			INSERT INTO oikumenea.tenant_graphs (org_id, code, name, is_authority_bearing)
			VALUES ($1, $2, 'Diff graph', $3) RETURNING id`,
			orgID, fmt.Sprintf("diff-%s-g%d", tag, gi), bearing).Scan(&gid); err != nil {
			t.Fatalf("seed graph: %v", err)
		}
		w.graphs = append(w.graphs, gid)
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				if r.Intn(n) < 2 { // sparse; multi-parent allowed
					if _, err := pool.Exec(ctx, `
						INSERT INTO oikumenea.tenant_unit_edges (graph_id, parent_id, child_id)
						VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, gid, w.units[i], w.units[j]); err != nil {
						t.Fatalf("seed edge: %v", err)
					}
				}
			}
		}
		if _, err := pool.Exec(ctx, `
			WITH RECURSIVE
			  nodes AS (
			    SELECT parent_id AS u FROM oikumenea.tenant_unit_edges WHERE graph_id = $1
			    UNION
			    SELECT child_id FROM oikumenea.tenant_unit_edges WHERE graph_id = $1
			  ),
			  reach AS (
			    SELECT u AS ancestor_id, u AS descendant_id, 0 AS depth FROM nodes
			    UNION ALL
			    SELECT rr.ancestor_id, e.child_id, rr.depth + 1
			    FROM reach rr
			    JOIN oikumenea.tenant_unit_edges e ON e.graph_id = $1 AND e.parent_id = rr.descendant_id
			  )
			INSERT INTO oikumenea.tenant_unit_closure (graph_id, ancestor_id, descendant_id, depth)
			SELECT $1::uuid, ancestor_id, descendant_id, min(depth)::int FROM reach
			GROUP BY ancestor_id, descendant_id`, gid); err != nil {
			t.Fatalf("rebuild closure: %v", err)
		}
	}

	nPersons := 2 + r.Intn(6)
	for i := 0; i < nPersons; i++ {
		var pid string
		if err := pool.QueryRow(ctx,
			`INSERT INTO oikumenea.person_persons (display_name) VALUES ($1) RETURNING id`,
			fmt.Sprintf("Diff %s p%d", tag, i)).Scan(&pid); err != nil {
			t.Fatalf("seed person: %v", err)
		}
		w.persons = append(w.persons, pid)
		for k := r.Intn(3); k > 0; k-- {
			u := w.units[r.Intn(n)]
			switch r.Intn(4) {
			case 0: // ended membership — must NOT contribute
				if _, err := pool.Exec(ctx, `
					INSERT INTO oikumenea.membership_memberships (person_id, unit_id, status, effective_to)
					VALUES ($1, $2, 'ended', now())`, pid, u); err != nil {
					t.Fatalf("seed ended membership: %v", err)
				}
			case 1: // soft-deleted membership — must NOT contribute
				if _, err := pool.Exec(ctx, `
					INSERT INTO oikumenea.membership_memberships (person_id, unit_id, deleted_at)
					VALUES ($1, $2, now())`, pid, u); err != nil {
					t.Fatalf("seed deleted membership: %v", err)
				}
			default:
				if _, err := pool.Exec(ctx, `
					INSERT INTO oikumenea.membership_memberships (person_id, unit_id)
					VALUES ($1, $2) ON CONFLICT DO NOTHING`, pid, u); err != nil {
					t.Fatalf("seed membership: %v", err)
				}
				w.activeIn[pid] = append(w.activeIn[pid], u)
			}
		}
	}

	nRoles := 1 + r.Intn(2)
	var roles []string
	for i := 0; i < nRoles; i++ {
		perms := map[string]bool{}
		for len(perms) == 0 {
			for _, p := range diffPermPool {
				if r.Intn(3) == 0 {
					perms[p] = true
				}
			}
		}
		var rid string
		if err := pool.QueryRow(ctx,
			`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Diff role') RETURNING id`,
			fmt.Sprintf("diff-%s-r%d", tag, i)).Scan(&rid); err != nil {
			t.Fatalf("seed role: %v", err)
		}
		for p := range perms {
			if _, err := pool.Exec(ctx,
				`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, $2)`, rid, p); err != nil {
				t.Fatalf("seed perm: %v", err)
			}
		}
		if r.Intn(6) == 0 { // soft-deleted role — its assignments must confer nothing
			if _, err := pool.Exec(ctx, `UPDATE oikumenea.authz_roles SET deleted_at = now() WHERE id = $1`, rid); err != nil {
				t.Fatalf("soft-delete role: %v", err)
			}
		}
		roles = append(roles, rid)
	}

	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ($1) RETURNING id`,
		"Diff subject "+tag).Scan(&w.subject); err != nil {
		t.Fatalf("seed subject: %v", err)
	}
	for k := r.Intn(7); k > 0; k-- {
		role := roles[r.Intn(len(roles))]
		target := w.units[r.Intn(n)]
		scope, graph := "unit", any(nil)
		if r.Intn(2) == 0 {
			scope, graph = "subtree", w.graphs[r.Intn(len(w.graphs))]
		}
		revoked, expires := any(nil), any(nil)
		if r.Intn(7) == 0 {
			revoked = time.Now().Add(-time.Hour)
		}
		switch r.Intn(7) {
		case 0:
			expires = time.Now().Add(-time.Hour) // expired — inactive at decision time
		case 1:
			expires = time.Now().Add(time.Hour) // future expiry — active
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO oikumenea.authz_role_assignments
			  (subject_person_id, role_id, target_unit_id, scope, graph_id, revoked_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`,
			w.subject, role, target, scope, graph, revoked, expires); err != nil {
			t.Fatalf("seed assignment: %v", err)
		}
	}
	return w
}
