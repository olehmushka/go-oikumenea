//go:build integration

// Integration tests for the generic link-traversal engine (review-2026-09 R-27 / D-LinkTraversal)
// against a real Postgres, exercising the ACTUAL descriptor + visibility + exemption wiring from
// link_descriptors.go (registerLinkDescriptors) — the review's acceptance criteria:
//
//   - COVERAGE (the drift guard, pairing R-28): MustBeBound passes, i.e. every kind=link type in the
//     pkg/rid registry is either registered or explicitly exempt; a link type that is neither fails.
//
//   - a person's memberships, kinships and (polymorphic) finance-account holdings come back grouped
//     in ONE call — the ~19-request client fan-out collapsed to O(1);
//
//   - a subject lacking a link's read permission gets NEITHER the link NOR the neighbor (per-arm gate);
//
//   - a neighbor person outside the subject's read reach is trimmed (D-VisibilityScope, R-30);
//
//   - polymorphic held_by resolves person vs company (tenant-org) neighbors correctly;
//
//   - the composite keyset token walks a multi-row arm without skips or duplicates.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./cmd/oikumenea/ -run TestLink
package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzadapters "github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	authzapp "github.com/olegamysk/go-oikumenea/internal/authorization/application"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	linksapp "github.com/olegamysk/go-oikumenea/internal/links/application"
	linksdomain "github.com/olegamysk/go-oikumenea/internal/links/domain"
	localizationadapters "github.com/olegamysk/go-oikumenea/internal/localization/adapters"
	localizationapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	localizationdomain "github.com/olegamysk/go-oikumenea/internal/localization/domain"
	membershipadapters "github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	membershipapp "github.com/olegamysk/go-oikumenea/internal/membership/application"
	membershipdomain "github.com/olegamysk/go-oikumenea/internal/membership/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/bearertoken"
)

const linksTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

type linkWorld struct {
	pool   *pgxpool.Pool
	authz  *authzapp.Service
	mem    *membershipapp.Service
	engine *linksapp.Service
}

func newLinkWorld(t *testing.T) linkWorld {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = linksTestDSN
	}
	return newLinkWorldDSN(t, dsn)
}

func newLinkWorldDSN(t *testing.T, dsn string) linkWorld {
	t.Helper()
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
	}, audit, authzdomain.NewPDP(tenantSvc), tenantSvc,
		func(conn pdb.DBTX) authzdomain.PrincipalRepository { return authzadapters.NewRepository(conn) })
	enforcer := pep.New(authzSvc)
	memSvc := membershipapp.NewService(pool, func(conn pdb.DBTX) membershipdomain.Repository {
		return membershipadapters.NewRepository(conn)
	}, audit)
	locSvc := localizationapp.NewService(pool, func(conn pdb.DBTX) localizationdomain.Repository {
		return localizationadapters.NewRepository(conn)
	}, audit)

	engine := linksapp.NewService(
		pool,
		func(ctx context.Context) (string, bool, error) { return enforcer.SubjectAuthority(ctx) },
		func(ctx context.Context, action string) (bool, error) {
			return enforcer.AllowedAnywhere(ctx, bearertoken.Token(""), action)
		},
	)
	// The ACTUAL composition wiring — descriptors + visibilities + exemptions.
	if err := registerLinkDescriptors(engine, pool, memSvc, authzSvc, locSvc); err != nil {
		t.Fatalf("registerLinkDescriptors: %v", err)
	}
	return linkWorld{pool: pool, authz: authzSvc, mem: memSvc, engine: engine}
}

func linksSubjectCtx(personID string) context.Context {
	return authn.NewContext(context.Background(), authn.Subject{PersonID: personID})
}

const linksEnsureOrgSQL = `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('lnk-domain','Link Domain')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'lnk-org','Link Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='lnk-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`

func (w linkWorld) orgID(t *testing.T) string {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), linksEnsureOrgSQL); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	var id string
	if err := w.pool.QueryRow(context.Background(),
		`SELECT id::text FROM oikumenea.tenant_organizations WHERE code='lnk-org'`).Scan(&id); err != nil {
		t.Fatalf("org id: %v", err)
	}
	return id
}

func (w linkWorld) seedUnit(t *testing.T) string {
	t.Helper()
	org := w.orgID(t)
	var id string
	if err := w.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (org_id, domain_id, code, name)
		 SELECT $1, o.domain_id, $2, 'Unit' FROM oikumenea.tenant_organizations o WHERE o.id=$1 RETURNING id::text`,
		org, "lnk-"+uuid.NewString()[:8]).Scan(&id); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return id
}

func (w linkWorld) seedPerson(t *testing.T, name, unitID string) string {
	t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ($1) RETURNING id::text`, name).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	if unitID != "" {
		if _, err := w.pool.Exec(context.Background(),
			`INSERT INTO oikumenea.membership_memberships (person_id, unit_id) VALUES ($1,$2)`, id, unitID); err != nil {
			t.Fatalf("seed membership: %v", err)
		}
	}
	return id
}

func (w linkWorld) seedKinship(t *testing.T, parent, child string) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(),
		`INSERT INTO oikumenea.person_kinships (parent_id, child_id) VALUES ($1,$2)`, parent, child); err != nil {
		t.Fatalf("seed kinship: %v", err)
	}
}

func (w linkWorld) seedFinanceAccount(t *testing.T) string {
	t.Helper()
	org := w.orgID(t)
	var id string
	if err := w.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.finance_accounts (institution_id, key_ref, iban_blind_index)
		 VALUES ($1, 'test-key', decode(md5(random()::text), 'hex')) RETURNING id::text`, org).Scan(&id); err != nil {
		t.Fatalf("seed finance account: %v", err)
	}
	return id
}

func (w linkWorld) seedHolder(t *testing.T, accountID, kind, holderRID, role string) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(),
		`INSERT INTO oikumenea.finance_account_holders (account_id, holder_kind, holder_id, role) VALUES ($1,$2,$3,$4)`,
		accountID, kind, holderRID, role); err != nil {
		t.Fatalf("seed holder: %v", err)
	}
}

func (w linkWorld) makeAdmin(t *testing.T, personID string) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(),
		`INSERT INTO oikumenea.authz_instance_admins (person_id) VALUES ($1)`, personID); err != nil {
		t.Fatalf("seed instance admin: %v", err)
	}
}

// seedGrant gives subject a fresh role with perms, subtree-scoped on unitID.
func (w linkWorld) seedGrant(t *testing.T, subjectID, unitID string, perms ...string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := w.pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1,'Link test role') RETURNING id`,
		"lnk-role-"+uuid.NewString()[:8]).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	for _, p := range perms {
		if _, err := w.pool.Exec(ctx,
			`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1,$2)`, roleID, p); err != nil {
			t.Fatalf("seed permission %s: %v", p, err)
		}
	}
	if _, err := w.pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
		 VALUES ($1,$2,$3,'unit')`, subjectID, roleID, unitID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
}

// groupsByType indexes GetObjectLinks output as linkType -> targetType -> []targetRID.
func collect(res linksdomain.ObjectLinks) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	for _, g := range res.Groups {
		if out[g.LinkType] == nil {
			out[g.LinkType] = map[string][]string{}
		}
		for _, r := range g.Rows {
			out[g.LinkType][r.TargetType] = append(out[g.LinkType][r.TargetType], r.TargetRID)
		}
	}
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestLinkCoverage is the drift guard: the real descriptor set + exemptions cover every kind=link
// type in the pkg/rid registry (MustBeBound). A migration adding a link type without wiring it here
// would fail this test — the executable form of "a new link table appears without touching web/".
func TestLinkCoverage(t *testing.T) {
	w := newLinkWorld(t)
	if err := w.engine.MustBeBound(); err != nil {
		t.Fatalf("coverage assertion failed: %v", err)
	}
}

// TestGetObjectLinksForPerson: memberships + kinships + polymorphic held_by come back grouped in one
// call for an instance admin (all arms pass, no trimming).
func TestGetObjectLinksForPerson(t *testing.T) {
	w := newLinkWorld(t)
	admin := w.seedPerson(t, "Admin", "")
	w.makeAdmin(t, admin)

	unit := w.seedUnit(t)
	p := w.seedPerson(t, "Subject Person", unit)
	child := w.seedPerson(t, "Child Person", unit)
	w.seedKinship(t, p, child)
	acct := w.seedFinanceAccount(t)
	w.seedHolder(t, acct, "person", p, "primary")

	res, err := w.engine.GetObjectLinks(linksSubjectCtx(admin), p, "", 0, "")
	if err != nil {
		t.Fatalf("GetObjectLinks: %v", err)
	}
	idx := collect(res)
	if !contains(idx["member_of"]["unit"], unit) {
		t.Errorf("member_of should include unit %s; got %v", unit, idx["member_of"])
	}
	if !contains(idx["kin_parent_of"]["person"], child) {
		t.Errorf("kin_parent_of should include child %s; got %v", child, idx["kin_parent_of"])
	}
	if !contains(idx["held_by"]["account"], acct) {
		t.Errorf("held_by should include account %s (person as poly source); got %v", acct, idx["held_by"])
	}
}

// TestNeighborLabels: neighbor rows carry a server-resolved locale→text display name (R-27 labeler
// seam) — person neighbors from their display_name, unit neighbors from tenant_units.name — instead of
// the empty label that made the client fall back to the RID tail.
func TestNeighborLabels(t *testing.T) {
	w := newLinkWorld(t)
	admin := w.seedPerson(t, "Admin", "")
	w.makeAdmin(t, admin)

	unit := w.seedUnit(t)
	p := w.seedPerson(t, "Subject Person", unit)
	child := w.seedPerson(t, "Kin Child", unit)
	w.seedKinship(t, p, child)

	res, err := w.engine.GetObjectLinks(linksSubjectCtx(admin), p, "", 0, "")
	if err != nil {
		t.Fatalf("GetObjectLinks: %v", err)
	}
	label := func(linkType, targetType, targetRID string) map[string]string {
		for _, g := range res.Groups {
			if g.LinkType != linkType || g.TargetType != targetType {
				continue
			}
			for _, r := range g.Rows {
				if r.TargetRID == targetRID {
					return r.Labels
				}
			}
		}
		return nil
	}
	// person neighbor (kin child) → display_name in some locale.
	kl := label("kin_parent_of", "person", child)
	if len(kl) == 0 {
		t.Fatalf("kin child neighbor has no label map")
	}
	if !hasValue(kl, "Kin Child") {
		t.Errorf("kin child label %v should contain display name %q", kl, "Kin Child")
	}
	// unit neighbor (membership) → tenant_units.name ("Unit").
	ul := label("member_of", "unit", unit)
	if len(ul) == 0 {
		t.Fatalf("unit neighbor has no label map")
	}
	if !hasValue(ul, "Unit") {
		t.Errorf("unit label %v should contain the unit name %q", ul, "Unit")
	}
}

func hasValue(m map[string]string, want string) bool {
	for _, v := range m {
		if v == want {
			return true
		}
	}
	return false
}

// TestPermissionGate: a subject with only membership.read sees member_of but NOT kin_parent_of
// (person.read) nor held_by (finance.read) — the per-arm gate.
func TestPermissionGate(t *testing.T) {
	w := newLinkWorld(t)
	unit := w.seedUnit(t)
	p := w.seedPerson(t, "Gate Subject", unit)
	child := w.seedPerson(t, "Gate Child", unit)
	w.seedKinship(t, p, child)
	acct := w.seedFinanceAccount(t)
	w.seedHolder(t, acct, "person", p, "primary")

	subject := w.seedPerson(t, "Viewer", unit)
	w.seedGrant(t, subject, unit, string(authzdomain.PermMembershipRead))

	res, err := w.engine.GetObjectLinks(linksSubjectCtx(subject), p, "", 0, "")
	if err != nil {
		t.Fatalf("GetObjectLinks: %v", err)
	}
	idx := collect(res)
	if !contains(idx["member_of"]["unit"], unit) {
		t.Errorf("member_of should be visible with membership.read; got %v", idx["member_of"])
	}
	if len(idx["kin_parent_of"]) != 0 {
		t.Errorf("kin_parent_of must be gated off without person.kinship.read; got %v", idx["kin_parent_of"])
	}
	if len(idx["held_by"]) != 0 {
		t.Errorf("held_by must be gated off without finance.read; got %v", idx["held_by"])
	}
}

// TestRelationshipCodeGate pins the D-LinkPermissions narrowing: the person relationship graph carries
// its OWN read code, so plain person.read (base unit-reader) no longer discloses who a person is
// related to — the person-relationship-reader grant does. This is the invariant that makes the per-link
// codes real rather than cosmetic; it is the same code the person module's ListKinships requires, so
// the object graph and the person page disclose the same set.
func TestRelationshipCodeGate(t *testing.T) {
	w := newLinkWorld(t)
	unit := w.seedUnit(t)
	p := w.seedPerson(t, "Rel Subject", unit)
	child := w.seedPerson(t, "Rel Child", unit)
	w.seedKinship(t, p, child)

	// A plain person reader: sees the person, NOT the relationship graph.
	reader := w.seedPerson(t, "Plain Reader", unit)
	w.seedGrant(t, reader, unit, string(authzdomain.PermPersonRead))
	res, err := w.engine.GetObjectLinks(linksSubjectCtx(reader), p, "kin_parent_of", 0, "")
	if err != nil {
		t.Fatalf("GetObjectLinks(reader): %v", err)
	}
	if got := collect(res)["kin_parent_of"]; len(got) != 0 {
		t.Errorf("person.read alone must NOT disclose the relationship graph; got %v", got)
	}

	// A relationship reader: the arm opens.
	relReader := w.seedPerson(t, "Rel Reader", unit)
	w.seedGrant(t, relReader, unit, string(authzdomain.PermPersonRead), string(authzdomain.PermPersonKinshipRead))
	res, err = w.engine.GetObjectLinks(linksSubjectCtx(relReader), p, "kin_parent_of", 0, "")
	if err != nil {
		t.Fatalf("GetObjectLinks(relReader): %v", err)
	}
	if kin := collect(res)["kin_parent_of"]["person"]; !contains(kin, child) {
		t.Errorf("person.kinship.read should disclose the kin child %s; got %v", child, kin)
	}
}

// TestNeighborTrim: with person.read but reach only to the subject's own unit, a kinship neighbor in
// an UNREACHABLE unit is trimmed out even though the arm gate passes (D-VisibilityScope, R-30).
func TestNeighborTrim(t *testing.T) {
	w := newLinkWorld(t)
	reachUnit := w.seedUnit(t)
	farUnit := w.seedUnit(t)
	p := w.seedPerson(t, "Trim Subject", reachUnit)
	near := w.seedPerson(t, "Near Kin", reachUnit)
	far := w.seedPerson(t, "Far Kin", farUnit)
	w.seedKinship(t, p, near)
	w.seedKinship(t, p, far)

	subject := w.seedPerson(t, "Trim Viewer", reachUnit)
	// person.read alone no longer opens the kin arm: the relationship graph carries its own code
	// (D-LinkPermissions, the person-relationship-reader role). This test is about the neighbor TRIM,
	// so grant the arm and assert the trim still bites.
	w.seedGrant(t, subject, reachUnit, string(authzdomain.PermPersonRead), string(authzdomain.PermPersonKinshipRead))

	res, err := w.engine.GetObjectLinks(linksSubjectCtx(subject), p, "kin_parent_of", 0, "")
	if err != nil {
		t.Fatalf("GetObjectLinks: %v", err)
	}
	kin := collect(res)["kin_parent_of"]["person"]
	if !contains(kin, near) {
		t.Errorf("reachable kin %s should be visible; got %v", near, kin)
	}
	if contains(kin, far) {
		t.Errorf("unreachable kin %s must be trimmed; got %v", far, kin)
	}
}

// TestPolymorphicHolder: a finance account with a person holder and a company (tenant-org) holder
// surfaces both, correctly typed, when expanding the account.
func TestPolymorphicHolder(t *testing.T) {
	w := newLinkWorld(t)
	admin := w.seedPerson(t, "Poly Admin", "")
	w.makeAdmin(t, admin)
	org := w.orgID(t)
	person := w.seedPerson(t, "Poly Person", "")
	acct := w.seedFinanceAccount(t)
	w.seedHolder(t, acct, "person", person, "primary")
	w.seedHolder(t, acct, "company", org, "joint")

	res, err := w.engine.GetObjectLinks(linksSubjectCtx(admin), acct, "", 0, "")
	if err != nil {
		t.Fatalf("GetObjectLinks: %v", err)
	}
	idx := collect(res)
	if !contains(idx["held_by"]["person"], person) {
		t.Errorf("held_by should include person holder %s; got %v", person, idx["held_by"])
	}
	if !contains(idx["held_by"]["organization"], org) {
		t.Errorf("held_by should include company/org holder %s; got %v", org, idx["held_by"])
	}
}

// hop2edge identifies one depth-2 result row as the edge it represents.
type hop2edge struct{ via, target string }

// walkDepth2 pages searchAround(depth=2) to exhaustion, returning the distinct hop-1 neighbor set and
// the list of hop-2 edges, asserting no duplicate edge is ever emitted across pages.
func walkDepth2(t *testing.T, w linkWorld, ctx context.Context, rid, linkTypes string, pageSize int) (map[string]bool, []hop2edge) {
	t.Helper()
	hop1 := map[string]bool{}
	var hop2 []hop2edge
	seenEdge := map[hop2edge]bool{}
	token := ""
	for pages := 0; pages < 200; pages++ {
		res, err := w.engine.SearchAroundDepth(ctx, rid, linkTypes, 2, pageSize, token)
		if err != nil {
			t.Fatalf("SearchAroundDepth page %d: %v", pages, err)
		}
		for _, n := range res.Neighbors {
			switch n.Hop {
			case 2:
				e := hop2edge{via: n.ViaRID, target: n.TargetRID}
				if seenEdge[e] {
					t.Errorf("duplicate hop-2 edge %+v across pages", e)
				}
				seenEdge[e] = true
				hop2 = append(hop2, e)
				if n.ViaRID == "" {
					t.Errorf("hop-2 row %s missing viaRid", n.TargetRID)
				}
			default: // hop 1 (0 or 1)
				hop1[n.TargetRID] = true
			}
		}
		if res.NextPageToken == "" {
			return hop1, hop2
		}
		token = res.NextPageToken
	}
	t.Fatalf("depth-2 walk did not terminate")
	return nil, nil
}

func hasEdge(edges []hop2edge, via, target string) bool {
	for _, e := range edges {
		if e.via == via && e.target == target {
			return true
		}
	}
	return false
}

// TestSearchAroundDepth2FrontierWalk: a two-generation kin graph expanded at depth 2 with a small page
// size returns every hop-1 child and every hop-2 grandchild edge, exhaustively, with no dup/skip —
// the "full keyset frontier" walk across a multi-node frontier.
func TestSearchAroundDepth2FrontierWalk(t *testing.T) {
	w := newLinkWorld(t)
	admin := w.seedPerson(t, "D2 Admin", "")
	w.makeAdmin(t, admin)

	root := w.seedPerson(t, "D2 Root", "")
	c1 := w.seedPerson(t, "D2 C1", "")
	c2 := w.seedPerson(t, "D2 C2", "")
	c3 := w.seedPerson(t, "D2 C3", "") // no grandchildren
	w.seedKinship(t, root, c1)
	w.seedKinship(t, root, c2)
	w.seedKinship(t, root, c3)
	g1a := w.seedPerson(t, "D2 G1a", "")
	g1b := w.seedPerson(t, "D2 G1b", "")
	g2a := w.seedPerson(t, "D2 G2a", "")
	w.seedKinship(t, c1, g1a)
	w.seedKinship(t, c1, g1b)
	w.seedKinship(t, c2, g2a)

	hop1, hop2 := walkDepth2(t, w, linksSubjectCtx(admin), root, "kin_parent_of", 2)

	for _, c := range []string{c1, c2, c3} {
		if !hop1[c] {
			t.Errorf("hop-1 child %s missing", c)
		}
	}
	for _, want := range []hop2edge{{c1, g1a}, {c1, g1b}, {c2, g2a}} {
		if !hasEdge(hop2, want.via, want.target) {
			t.Errorf("hop-2 edge %+v missing; got %v", want, hop2)
		}
	}
	// c3 has no children ⇒ contributes no hop-2 edge.
	if hasEdge(hop2, c3, "") || len(hop2) != 3 {
		t.Errorf("expected exactly 3 hop-2 edges, got %v", hop2)
	}
}

// TestSearchAroundDepth2Backtrack: the trivial backtrack edge to the origin is excluded, but a genuine
// alternate 2-path is kept — a grandchild that is ALSO a direct child appears both as a hop-1 neighbor
// and as a hop-2 edge through the intermediate node (each row is an edge, not a deduped node).
func TestSearchAroundDepth2Backtrack(t *testing.T) {
	w := newLinkWorld(t)
	admin := w.seedPerson(t, "BT Admin", "")
	w.makeAdmin(t, admin)

	root := w.seedPerson(t, "BT Root", "")
	c1 := w.seedPerson(t, "BT C1", "")
	c2 := w.seedPerson(t, "BT C2", "")
	w.seedKinship(t, root, c1)
	w.seedKinship(t, root, c2)
	w.seedKinship(t, c1, c2) // c2 is a child of both root (direct) and c1 (via c1)

	hop1, hop2 := walkDepth2(t, w, linksSubjectCtx(admin), root, "kin_parent_of", 50)

	if !hop1[c1] || !hop1[c2] {
		t.Errorf("both direct children expected at hop-1; got %v", hop1)
	}
	if !hasEdge(hop2, c1, c2) {
		t.Errorf("alternate 2-path (c1→c2) must be kept as a hop-2 edge; got %v", hop2)
	}
	// The origin must never appear as a hop-2 neighbor (c1/c2 point back at root via the in-arm).
	if hasEdge(hop2, c1, root) || hasEdge(hop2, c2, root) {
		t.Errorf("origin %s must be excluded from hop-2 (backtrack); got %v", root, hop2)
	}
}

// TestSearchAroundDepth2PerHopGate: restricted to the kin arm, a subject without the kinship read code
// sees NOTHING at either hop (the arm is gated off, so the hop-1 child is never a frontier node and its
// grandchild is never reached); a kinship reader sees both. The arm gate is applied at every hop.
func TestSearchAroundDepth2PerHopGate(t *testing.T) {
	w := newLinkWorld(t)
	unit := w.seedUnit(t)
	root := w.seedPerson(t, "Gate2 Root", unit)
	c := w.seedPerson(t, "Gate2 Child", unit)
	g := w.seedPerson(t, "Gate2 Grandchild", unit)
	w.seedKinship(t, root, c)
	w.seedKinship(t, c, g)

	// membership.read only ⇒ the kin arm is gated off at both hops.
	gated := w.seedPerson(t, "Gate2 Gated", unit)
	w.seedGrant(t, gated, unit, string(authzdomain.PermMembershipRead))
	hop1, hop2 := walkDepth2(t, w, linksSubjectCtx(gated), root, "kin_parent_of", 50)
	if len(hop1) != 0 || len(hop2) != 0 {
		t.Errorf("kin arm must be fully gated without kinship.read; got hop1=%v hop2=%v", hop1, hop2)
	}

	// A kinship reader: the child (hop-1) and grandchild (hop-2 via child) both appear — the positive
	// control that the graph is non-empty and the gate, not the topology, produced the empty result.
	reader := w.seedPerson(t, "Gate2 Reader", unit)
	w.seedGrant(t, reader, unit, string(authzdomain.PermPersonRead), string(authzdomain.PermPersonKinshipRead))
	hop1, hop2 = walkDepth2(t, w, linksSubjectCtx(reader), root, "kin_parent_of", 50)
	if !hop1[c] {
		t.Errorf("kinship reader should see hop-1 child %s; got %v", c, hop1)
	}
	if !hasEdge(hop2, c, g) {
		t.Errorf("kinship reader should see hop-2 edge (child→grandchild); got %v", hop2)
	}
}

// TestSearchAroundDepth2Trim: a hop-1 kin child in an UNREACHABLE unit is trimmed from the frontier
// (D-VisibilityScope, R-30), so it is never expanded and its own grandchild never surfaces; the
// reachable child and its grandchild do.
func TestSearchAroundDepth2Trim(t *testing.T) {
	w := newLinkWorld(t)
	reachUnit := w.seedUnit(t)
	farUnit := w.seedUnit(t)
	root := w.seedPerson(t, "Trim2 Root", reachUnit)
	near := w.seedPerson(t, "Trim2 Near", reachUnit)
	far := w.seedPerson(t, "Trim2 Far", farUnit)
	w.seedKinship(t, root, near)
	w.seedKinship(t, root, far)
	gNear := w.seedPerson(t, "Trim2 GNear", reachUnit)
	gFar := w.seedPerson(t, "Trim2 GFar", reachUnit)
	w.seedKinship(t, near, gNear)
	w.seedKinship(t, far, gFar)

	subject := w.seedPerson(t, "Trim2 Viewer", reachUnit)
	w.seedGrant(t, subject, reachUnit, string(authzdomain.PermPersonRead), string(authzdomain.PermPersonKinshipRead))

	hop1, hop2 := walkDepth2(t, w, linksSubjectCtx(subject), root, "kin_parent_of", 50)

	if !hop1[near] {
		t.Errorf("reachable hop-1 child %s should be present; got %v", near, hop1)
	}
	if hop1[far] {
		t.Errorf("unreachable hop-1 child %s must be trimmed; got %v", far, hop1)
	}
	if !hasEdge(hop2, near, gNear) {
		t.Errorf("reachable grandchild edge (near→gNear) expected; got %v", hop2)
	}
	if hasEdge(hop2, far, gFar) {
		t.Errorf("trimmed hop-1 node %s must not be expanded to its grandchild; got %v", far, hop2)
	}
}

// TestSearchAroundDepth2InvalidToken: a depth-1 (v1) token is rejected on a depth-2 request — depth
// crossings never share a page token.
func TestSearchAroundDepth2InvalidToken(t *testing.T) {
	w := newLinkWorld(t)
	admin := w.seedPerson(t, "Tok Admin", "")
	w.makeAdmin(t, admin)
	root := w.seedPerson(t, "Tok Root", "")
	for i := 0; i < 3; i++ {
		w.seedKinship(t, root, w.seedPerson(t, "Tok Kin", ""))
	}
	// A depth-1 page with a small size yields a v1 continuation token.
	d1, err := w.engine.SearchAroundDepth(linksSubjectCtx(admin), root, "kin_parent_of", 1, 1, "")
	if err != nil {
		t.Fatalf("depth-1 page: %v", err)
	}
	if d1.NextPageToken == "" {
		t.Fatalf("expected a v1 continuation token from the depth-1 page")
	}
	if _, err := w.engine.SearchAroundDepth(linksSubjectCtx(admin), root, "kin_parent_of", 2, 50, d1.NextPageToken); err == nil {
		t.Errorf("a v1 token must be rejected on a depth-2 request")
	}
}

// TestSearchAroundDepth2Scale is the review-2026-09 gate measurement (the "< 1 s, 50-neighbor node,
// 2-hop, M49-scale dataset" acceptance, review-2026-09.md §R-27). It is SKIPPED unless
// OIKUMENEA_SCALE_DSN points at a seed-scale database (scripts/seed-scale: 100k units / 1M persons /
// 1M memberships). Because that harness seeds a shallow star (~1 membership/person, ≤27 members/unit),
// it attaches one probe person to 50 existing units so the origin has a genuine 50-node hop-1 frontier,
// each expanded one more hop into its members — the case depth-2 exists to serve. Measured with an
// admin viewer so the numbers isolate the NEW depth-2 machinery (hop-1 collect + distinct-neighbor
// frontier enumeration + one inner collect per frontier node); the per-hop visibility trim it
// short-circuits for admins is the identical bounded semi-join depth-1 already runs (R-30).
//
//	OIKUMENEA_SCALE_DSN="postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable" \
//	go test -tags integration ./cmd/oikumenea/ -run TestSearchAroundDepth2Scale -v
func TestSearchAroundDepth2Scale(t *testing.T) {
	dsn := os.Getenv("OIKUMENEA_SCALE_DSN")
	if dsn == "" {
		t.Skip("set OIKUMENEA_SCALE_DSN to a seed-scale database to run the depth-2 gate measurement")
	}
	w := newLinkWorldDSN(t, dsn)
	ctx := context.Background()

	// Probe person + admin viewer, both cleaned up so the scale DB stays pristine.
	var probe, admin string
	if err := w.pool.QueryRow(ctx,
		`INSERT INTO oikumenea.person_persons (code, display_name) VALUES ('d2-scale-probe','D2 Scale Probe') RETURNING id::text`).Scan(&probe); err != nil {
		t.Fatalf("seed probe: %v", err)
	}
	if err := w.pool.QueryRow(ctx,
		`INSERT INTO oikumenea.person_persons (code, display_name) VALUES ('d2-scale-admin','D2 Scale Admin') RETURNING id::text`).Scan(&admin); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	w.makeAdmin(t, admin)
	t.Cleanup(func() {
		_, _ = w.pool.Exec(ctx, `DELETE FROM oikumenea.membership_memberships WHERE person_id=$1`, probe)
		_, _ = w.pool.Exec(ctx, `DELETE FROM oikumenea.authz_instance_admins WHERE person_id=$1`, admin)
		_, _ = w.pool.Exec(ctx, `DELETE FROM oikumenea.person_persons WHERE id = ANY($1)`, []string{probe, admin})
	})

	// Attach the probe to 50 existing units that already carry members (so hop-2 does real work).
	var attached int
	if err := w.pool.QueryRow(ctx, `
		WITH targets AS (
			SELECT unit_id FROM oikumenea.membership_memberships
			GROUP BY unit_id HAVING count(*) >= 10 LIMIT 50)
		INSERT INTO oikumenea.membership_memberships (person_id, unit_id)
		SELECT $1, unit_id FROM targets
		RETURNING (SELECT count(*) FROM targets)`, probe).Scan(&attached); err != nil {
		t.Fatalf("attach probe memberships: %v", err)
	}
	if attached < 50 {
		t.Fatalf("expected 50 probe memberships, attached %d", attached)
	}

	// Walk depth-2 to exhaustion, timing the whole walk and counting DB statements.
	cctx, counter := pdb.WithQueryCounter(linksSubjectCtx(admin))
	start := time.Now()
	hop1, hop2, pages := 0, 0, 0
	token := ""
	for {
		res, err := w.engine.SearchAroundDepth(cctx, probe, "", 2, 200, token)
		if err != nil {
			t.Fatalf("SearchAroundDepth: %v", err)
		}
		pages++
		for _, n := range res.Neighbors {
			if n.Hop == 2 {
				hop2++
			} else {
				hop1++
			}
		}
		if res.NextPageToken == "" {
			break
		}
		token = res.NextPageToken
	}
	elapsed := time.Since(start)

	t.Logf("depth-2 @ M49 scale: hop1=%d hop2=%d total=%d pages=%d db_queries=%d elapsed=%s",
		hop1, hop2, hop1+hop2, pages, counter.Count(), elapsed)
	if hop1 == 0 || hop2 == 0 {
		t.Fatalf("probe produced no 2-hop neighborhood (hop1=%d hop2=%d)", hop1, hop2)
	}
	if elapsed >= time.Second {
		t.Fatalf("depth-2 gate NOT cleared: 50-node 2-hop walk took %s (>= 1s)", elapsed)
	}
}

// TestPagination: many kinships, small page size, token walk collects them all with no dup/skip.
func TestPagination(t *testing.T) {
	w := newLinkWorld(t)
	admin := w.seedPerson(t, "Page Admin", "")
	w.makeAdmin(t, admin)
	p := w.seedPerson(t, "Page Subject", "")
	want := map[string]bool{}
	for i := 0; i < 7; i++ {
		c := w.seedPerson(t, "Page Kin", "")
		w.seedKinship(t, p, c)
		want[c] = true
	}
	got := map[string]bool{}
	token := ""
	for pages := 0; pages < 20; pages++ {
		res, err := w.engine.GetObjectLinks(linksSubjectCtx(admin), p, "kin_parent_of", 2, token)
		if err != nil {
			t.Fatalf("GetObjectLinks page: %v", err)
		}
		for _, rid := range collect(res)["kin_parent_of"]["person"] {
			if got[rid] {
				t.Errorf("duplicate neighbor %s across pages", rid)
			}
			got[rid] = true
		}
		if res.NextPageToken == "" {
			break
		}
		token = res.NextPageToken
	}
	for c := range want {
		if !got[c] {
			t.Errorf("kin %s missing from paginated walk", c)
		}
	}
}
