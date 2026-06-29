//go:build integration

// Integration tests for the Religion core vertical against a real Postgres (M22 exit criteria,
// D-Religion / D-Audit). They exercise the recursive taxonomy + closure (filter, effective theism
// resolution with nearest-declared-wins + override, reparent cycle guard), the per-unit organization
// attributes (profile + M:N classifications/one-primary), the effective-type resolution + unit
// override, and the data-driven excludes_child_creation policy blocking createChildOrg.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/religion/...
package religion_test

import (
	"bytes"
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
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/religion/adapters"
	"github.com/olegamysk/go-oikumenea/internal/religion/application"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
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
	tenantSvc := tenantapp.NewService(pool, func(conn pdb.DBTX) tenantdomain.Repository {
		return tenantadapters.NewRepository(conn)
	}, audit)
	return application.NewService(pool, func(conn pdb.DBTX) application.Repo {
		return adapters.NewRepository(conn)
	}, audit, tenantSvc, testCipher(t))
}

func testCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	kek := make([]byte, 32)
	provider, err := crypto.NewLocalDevProvider(kek)
	if err != nil {
		t.Fatalf("local-dev key provider: %v", err)
	}
	cipher, err := crypto.NewCipher(provider, []byte("integration-blind-index-key"), 0)
	if err != nil {
		t.Fatalf("build cipher: %v", err)
	}
	return cipher
}

func seedPerson(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

func gradeID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.religion_clergy_grades WHERE code=$1 AND deleted_at IS NULL", code).Scan(&id); err != nil {
		t.Fatalf("resolve grade %s: %v", code, err)
	}
	return id
}

func affiliationTypeID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.religion_affiliation_types WHERE code=$1 AND deleted_at IS NULL", code).Scan(&id); err != nil {
		t.Fatalf("resolve affiliation type %s: %v", code, err)
	}
	return id
}

func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func taxonID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.religion_taxa WHERE code=$1 AND deleted_at IS NULL", code).Scan(&id); err != nil {
		t.Fatalf("resolve taxon %s: %v", code, err)
	}
	return id
}

func classificationID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.religion_classifications WHERE code=$1", code).Scan(&id); err != nil {
		t.Fatalf("resolve classification %s: %v", code, err)
	}
	return id
}

func hasCode(cls []domain.Classification, code string) bool {
	for _, c := range cls {
		if c.Code == code {
			return true
		}
	}
	return false
}

// TestTaxonomySeedAndResolution proves the curated seed loaded and the nearest-declared-wins theism
// resolution walks the closure (a denomination inherits its root religion's type unless it overrides).
func TestTaxonomySeedAndResolution(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	religions, err := svc.ListTaxa(ctx, "religion", "", "", "", "", 100)
	if err != nil {
		t.Fatalf("list religions: %v", err)
	}
	if len(religions) < 12 {
		t.Fatalf("expected the seeded world religions, got %d", len(religions))
	}

	// Filter by religion CODE (regression: the param must not be compared against the uuid column).
	christianTaxa, err := svc.ListTaxa(ctx, "", "", "christianity", "", "", 300)
	if err != nil {
		t.Fatalf("list by religion code: %v", err)
	}
	if len(christianTaxa) < 30 {
		t.Fatalf("expected the Christian subtree, got %d", len(christianTaxa))
	}
	// A childless root religion resolves to just itself (religion_id = its own id).
	zoro, err := svc.ListTaxa(ctx, "", "", "zoroastrianism", "", "", 50)
	if err != nil {
		t.Fatalf("list by childless religion: %v", err)
	}
	if len(zoro) < 1 {
		t.Fatalf("expected zoroastrianism itself, got %d", len(zoro))
	}

	// Atheism/agnosticism are seeded as pickable root religions, each tagged with its theism class.
	atheism := taxonID(t, pool, "atheism")
	atheismEff, err := svc.EffectiveClassifications(ctx, atheism)
	if err != nil {
		t.Fatalf("atheism effective classifications: %v", err)
	}
	if !hasCode(atheismEff, "atheistic") {
		t.Fatalf("atheism should resolve to atheistic, got %v", atheismEff)
	}
	agnosticism := taxonID(t, pool, "agnosticism")
	agnEff, err := svc.EffectiveClassifications(ctx, agnosticism)
	if err != nil {
		t.Fatalf("agnosticism effective classifications: %v", err)
	}
	if !hasCode(agnEff, "agnostic") {
		t.Fatalf("agnosticism should resolve to agnostic, got %v", agnEff)
	}

	// A denomination deep under Christianity inherits 'monotheistic' from the root religion.
	ocu := taxonID(t, pool, "orthodox_church_of_ukraine")
	eff, err := svc.EffectiveClassifications(ctx, ocu)
	if err != nil {
		t.Fatalf("effective classifications: %v", err)
	}
	if !hasCode(eff, "monotheistic") {
		t.Fatalf("OCU should inherit monotheistic, got %v", eff)
	}

	// Override at the denomination level wins over the inherited religion-level set.
	if _, err := svc.SetTaxonClassifications(ctx, ocu, []string{classificationID(t, pool, "dualistic")}); err != nil {
		t.Fatalf("set taxon classifications: %v", err)
	}
	t.Cleanup(func() { _, _ = svc.SetTaxonClassifications(ctx, ocu, nil) })
	eff, err = svc.EffectiveClassifications(ctx, ocu)
	if err != nil {
		t.Fatalf("effective after override: %v", err)
	}
	if !hasCode(eff, "dualistic") || hasCode(eff, "monotheistic") {
		t.Fatalf("override should yield only dualistic, got %v", eff)
	}
}

// TestReparentCycleGuard proves reparenting under a descendant is rejected.
func TestReparentCycleGuard(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	christianity := taxonID(t, pool, "christianity")

	rankID := func(code string) string {
		var id string
		if err := pool.QueryRow(ctx, "SELECT id FROM oikumenea.religion_taxon_ranks WHERE code=$1", code).Scan(&id); err != nil {
			t.Fatalf("rank %s: %v", code, err)
		}
		return id
	}

	a, err := svc.CreateTaxon(ctx, domain.TaxonInput{Code: uniq("br-a"), Name: "A", RankID: rankID("branch"), ParentID: christianity})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := svc.CreateTaxon(ctx, domain.TaxonInput{Code: uniq("br-b"), Name: "B", RankID: rankID("tradition"), ParentID: a.ID})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	t.Cleanup(func() { _ = svc.DeleteTaxon(ctx, b.ID); _ = svc.DeleteTaxon(ctx, a.ID) })

	if _, err := svc.ReparentTaxon(ctx, a.ID, b.ID); !errors.Is(err, domain.ErrTaxonCycle) {
		t.Fatalf("expected ErrTaxonCycle reparenting A under its descendant B, got %v", err)
	}
}

// TestOrgProfileAndEffectiveType proves the per-unit profile + M:N classifications (one primary), the
// effective-type resolution from the primary taxon, and the unit override.
func TestOrgProfileAndEffectiveType(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	unit := seedUnit(t, pool)
	if _, err := svc.SetOrgProfile(ctx, unit, nil, ptr("OCU")); err != nil {
		t.Fatalf("set profile: %v", err)
	}
	// two classifications, one primary (Eastern Orthodoxy primary + a secondary tag).
	primary := taxonID(t, pool, "orthodox_church_of_ukraine")
	secondary := taxonID(t, pool, "eastern_orthodoxy")
	if _, err := svc.AddOrgClassification(ctx, unit, primary, true, nil, nil); err != nil {
		t.Fatalf("add primary: %v", err)
	}
	if _, err := svc.AddOrgClassification(ctx, unit, secondary, false, nil, nil); err != nil {
		t.Fatalf("add secondary: %v", err)
	}
	prof, err := svc.GetOrgProfile(ctx, unit)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if len(prof.Classifications) != 2 {
		t.Fatalf("expected 2 classifications, got %d", len(prof.Classifications))
	}

	// effective-type derives from the primary taxon (inherited monotheistic from Christianity).
	et, err := svc.EffectiveType(ctx, unit)
	if err != nil {
		t.Fatalf("effective type: %v", err)
	}
	if et.Source == "none" || !hasCode(et.Classifications, "monotheistic") {
		t.Fatalf("expected taxon-sourced monotheistic, got source=%q %v", et.Source, et.Classifications)
	}

	// a unit override beats the inherited taxon set.
	if _, err := svc.SetUnitTypeOverride(ctx, unit, []string{classificationID(t, pool, "nontheistic")}); err != nil {
		t.Fatalf("set override: %v", err)
	}
	et, err = svc.EffectiveType(ctx, unit)
	if err != nil {
		t.Fatalf("effective type after override: %v", err)
	}
	if et.Source != "unit" || !hasCode(et.Classifications, "nontheistic") {
		t.Fatalf("expected unit-sourced nontheistic, got source=%q %v", et.Source, et.Classifications)
	}
}

// TestChildCreationPolicy proves the excludes_child_creation policy blocks createChildOrg, and removing
// it lets a child org be created in the canonical graph.
func TestChildCreationPolicy(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	parent := seedUnit(t, pool)
	if _, err := svc.SetOrgProfile(ctx, parent, nil, nil); err != nil {
		t.Fatalf("set parent profile: %v", err)
	}
	var policyKind string
	if err := pool.QueryRow(ctx, "SELECT id FROM oikumenea.religion_policy_kinds WHERE code='excludes_child_creation'").Scan(&policyKind); err != nil {
		t.Fatalf("resolve policy kind: %v", err)
	}
	pol, err := svc.AddOrgPolicy(ctx, parent, policyKind, ptr("closed body"), nil)
	if err != nil {
		t.Fatalf("add policy: %v", err)
	}

	_, err = svc.CreateChildOrg(ctx, parent, uniq("child"), "Child Parish", "", "", "")
	if !errors.Is(err, domain.ErrChildCreationExcluded) {
		t.Fatalf("expected ErrChildCreationExcluded, got %v", err)
	}

	if err := svc.RemoveOrgPolicy(ctx, parent, pol.ID); err != nil {
		t.Fatalf("remove policy: %v", err)
	}
	prof, err := svc.CreateChildOrg(ctx, parent, uniq("child"), "Child Parish", "", "", taxonID(t, pool, "eastern_orthodoxy"))
	if err != nil {
		t.Fatalf("create child org after policy removed: %v", err)
	}
	if prof.UnitID == "" {
		t.Fatalf("expected a created child unit profile")
	}
	// the child is in the canonical graph under the parent.
	var edges int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM oikumenea.tenant_unit_edges e
		JOIN oikumenea.tenant_graphs g ON g.id = e.graph_id
		WHERE g.code='canonical' AND e.parent_id=$1 AND e.child_id=$2`, parent, prof.UnitID).Scan(&edges); err != nil {
		t.Fatalf("check edge: %v", err)
	}
	if edges != 1 {
		t.Fatalf("expected 1 canonical edge parent->child, got %d", edges)
	}
}

// TestCreateRootOrg proves the first-class top-level-body path (M41 / D-UnifiedOrgGraph): CreateRootOrg
// builds a `church`-domain organization + its root religious-body unit + profile, and a child added under
// it via CreateChildOrg lands in the canonical graph — no hand-seeded org/unit fixtures.
func TestCreateRootOrg(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	ensureChurchDomain(t, pool)
	ocu := taxonID(t, pool, "orthodox_church_of_ukraine")

	root, err := svc.CreateRootOrg(ctx, uniq("ocu"), "Orthodox Church of Ukraine", "", "", ocu)
	if err != nil {
		t.Fatalf("create root org: %v", err)
	}
	if root.UnitID == "" {
		t.Fatalf("expected a root unit profile")
	}
	// The root unit lives in a `church`-domain organization (M40/M41).
	var domCode string
	if err := pool.QueryRow(ctx, `
		SELECT d.code FROM oikumenea.tenant_units u
		JOIN oikumenea.tenant_organizations o ON o.id = u.org_id
		JOIN oikumenea.tenant_domains d ON d.id = o.domain_id
		WHERE u.id = $1`, root.UnitID).Scan(&domCode); err != nil {
		t.Fatalf("resolve root org domain: %v", err)
	}
	if domCode != "church" {
		t.Fatalf("expected root body in church domain, got %q", domCode)
	}
	// A child body added under the root lands in the canonical graph.
	child, err := svc.CreateChildOrg(ctx, root.UnitID, uniq("eparchy"), "Kyiv Eparchy", "", "", "")
	if err != nil {
		t.Fatalf("create child under root: %v", err)
	}
	var edges int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM oikumenea.tenant_unit_edges e
		JOIN oikumenea.tenant_graphs g ON g.id = e.graph_id
		WHERE g.code='canonical' AND e.parent_id=$1 AND e.child_id=$2`, root.UnitID, child.UnitID).Scan(&edges); err != nil {
		t.Fatalf("check edge: %v", err)
	}
	if edges != 1 {
		t.Fatalf("expected 1 canonical edge root->child, got %d", edges)
	}
}

// TestClergyCredentialLifecycle proves the public clergy credential: add → list (by person + by unit) →
// status flip (suspend), and the indelible nature (no hard delete, status reflects revocation).
func TestClergyCredentialLifecycle(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	person := seedPerson(t, pool, "Rev. Test")
	unit := seedUnit(t, pool)
	bishop := gradeID(t, pool, "bishop")
	granted := "2020-05-01"

	cred, err := svc.AddClergyCredential(ctx, domain.ClergyCredentialInput{
		PersonID: person, ClergyGradeID: bishop, OrgUnitID: unit, GrantedOn: mustDate(t, granted),
	})
	if err != nil {
		t.Fatalf("add credential: %v", err)
	}
	if cred.Status != "active" || cred.GradeCode != "bishop" {
		t.Fatalf("unexpected credential %+v", cred)
	}

	byPerson, err := svc.ListPersonClergyCredentials(ctx, person)
	if err != nil || len(byPerson) != 1 {
		t.Fatalf("list by person: %v (n=%d)", err, len(byPerson))
	}
	byUnit, err := svc.ListUnitClergyCredentials(ctx, unit)
	if err != nil || len(byUnit) != 1 {
		t.Fatalf("list by unit: %v (n=%d)", err, len(byUnit))
	}

	suspended := "suspended"
	upd, err := svc.UpdateClergyCredential(ctx, cred.ID, domain.ClergyCredentialUpdate{Status: &suspended})
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if upd.Status != "suspended" {
		t.Fatalf("expected suspended, got %q", upd.Status)
	}
	// an invalid status is rejected.
	bad := "deleted"
	if _, err := svc.UpdateClergyCredential(ctx, cred.ID, domain.ClergyCredentialUpdate{Status: &bad}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected ErrInvalid for bad status, got %v", err)
	}
}

// TestAffiliationEncryption proves the pii:special envelope: a created affiliation's belief value is
// ENCRYPTED at rest (no plaintext in value_ciphertext; blind index present), reads round-trip the
// decrypted value, and ErasePersonAffiliations crypto-erases (drops the envelope, keeps the row).
func TestAffiliationEncryption(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	person := seedPerson(t, pool, "Lay Member")
	baptized := affiliationTypeID(t, pool, "baptized")
	christianity := taxonID(t, pool, "christianity")
	secret := "confirmed 2015 at St. Mary's, Lviv"

	aff, err := svc.AddAffiliation(ctx, domain.AffiliationInput{
		PersonID: person, AffiliationTypeID: baptized, ReligionID: christianity,
	}, secret)
	if err != nil {
		t.Fatalf("add affiliation: %v", err)
	}
	if aff.Value != secret {
		t.Fatalf("expected the value echoed back, got %q", aff.Value)
	}

	// At rest: ciphertext present, does NOT contain the plaintext; blind index present.
	var ciphertext, blind []byte
	if err := pool.QueryRow(ctx,
		"SELECT value_ciphertext, value_blind_index FROM oikumenea.religion_affiliations WHERE id=$1", aff.ID).Scan(&ciphertext, &blind); err != nil {
		t.Fatalf("read at rest: %v", err)
	}
	if len(ciphertext) == 0 || len(blind) == 0 {
		t.Fatalf("expected ciphertext + blind index at rest")
	}
	if bytes.Contains(ciphertext, []byte(secret)) {
		t.Fatalf("plaintext leaked into ciphertext column")
	}

	// Read round-trips the decrypted value.
	rows, err := svc.ListPersonAffiliations(ctx, person)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list affiliations: %v (n=%d)", err, len(rows))
	}
	if rows[0].Value != secret || rows[0].AffiliationTypeCode != "baptized" {
		t.Fatalf("decrypt round-trip failed: %+v", rows[0])
	}

	// Crypto-erase drops the envelope; the row survives as a tombstone with an empty value.
	n, err := svc.ErasePersonAffiliations(ctx, person)
	if err != nil || n != 1 {
		t.Fatalf("erase: %v (n=%d)", err, n)
	}
	after, err := svc.ListPersonAffiliations(ctx, person)
	if err != nil || len(after) != 1 {
		t.Fatalf("list after erase: %v (n=%d)", err, len(after))
	}
	if after[0].Value != "" {
		t.Fatalf("expected empty value after crypto-erase, got %q", after[0].Value)
	}
}

func mustDate(t *testing.T, s string) *time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return &d
}

func ptr(s string) *string { return &s }

// ensureOrgSQL idempotently seeds a church-domain organization (D-TenantOrganizations, M40) so a
// religious-body root unit can be placed in a real organization; child bodies inherit its org via
// CreateChildOrg. The canonical/tradition/affiliation graphs stay instance-global (migration-seeded).
const ensureOrgSQL = `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('test-church','Church')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'test-religion-org','Test Religion Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='test-church'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`

// ensureChurchDomain idempotently seeds the real `church` operational domain (pdp_scoped=true) — what
// tenant.Register seeds at boot — so CreateRootOrg can resolve it (M41 / D-UnifiedOrgGraph).
func ensureChurchDomain(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO oikumenea.tenant_domains (code, name, pdp_scoped)
		VALUES ('church','Church',true)
		ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("ensure church domain: %v", err)
	}
}

func seedUnit(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	if _, err := pool.Exec(context.Background(), ensureOrgSQL); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (org_id, domain_id, code, name)
		 SELECT o.id, o.domain_id, $1, 'Religious Body' FROM oikumenea.tenant_organizations o WHERE o.code='test-religion-org'
		 RETURNING id`, uniq("unit")).Scan(&id); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return id
}
