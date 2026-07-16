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
	}, audit, authzdomain.NewPDP(tenantSvc), tenantSvc)
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
