// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The SQL narg-parity guard (M56 / D-ObjectFacets). A person facet block is written into FIVE
// queries — person's two instance-admin queries and membership's three read-scope visibility
// queries — because the predicates must run inside each query, before the LIMIT (review-2026-07
// R-06). Five copies is five chances to drift, and the failure would be silent and asymmetric: an
// admin and a scoped caller applying the same filter would get different people.
//
// This test parses the .sql sources and asserts every facet's bound parameter appears in ALL of
// them. It needs no database, so it runs in the ordinary unit sweep.

// personFilterQueries are the five queries that must carry the identical person facet block. Named
// explicitly (rather than discovered) so DELETING a query's block fails here instead of shrinking
// the checked set to nothing.
var personFilterQueries = []struct{ module, query string }{
	{"person", "ListPersons"},
	{"person", "SearchPersons"},
	{"membership", "VisiblePersonIDsForSubjectSparse"},
	{"membership", "VisiblePersonIDsForSubjectDense"},
	{"membership", "VisiblePersonIDsForSubjectSearch"},
	// M57: the dashboard aggregates carry the SAME block. A list and a dashboard that disagree about
	// what a filter selects is the worst of the drift shapes — the chart would describe a set the
	// list does not return, and neither number would look wrong on its own.
	{"person", "PersonStats"},
	{"person", "PersonStatsSearch"},
	{"membership", "VisiblePersonStatsForSubject"},
	{"membership", "VisiblePersonStatsForSubjectSearch"},
}

// personFacetNargs maps each person facet key to the sqlc.narg name(s) its predicate binds. The
// SQL name is snake_case and occasionally prefixed (filter_unit_id, not unit_id, to avoid colliding
// with the membership queries' own unit_id columns), so the mapping is written down here and this
// test is what keeps it true.
var personFacetNargs = map[string][]string{
	"sex":            {"sex"},
	"status":         {"status"},
	"birthdate":      {"birthdate_from", "birthdate_to"},
	"countryOfBirth": {"country_of_birth_id"},
	"rankId":         {"rank_id"},
	"unitId":         {"filter_unit_id"},
	"hasAccount":     {"has_account"},
}

// unitFilterQueries: the unit facets live in a single flat-listing query (the traversal modes
// deliberately ignore them), so parity there is a presence check rather than a cross-query one.
var unitFilterQueries = []struct{ module, query string }{
	{"tenant", "ListUnits"},
	{"tenant", "UnitStats"},
	{"tenant", "UnitStatsForSubject"},
}

var unitFacetNargs = map[string][]string{
	"domain":     {"domain_id"},
	"unitKind":   {"kind_id"},
	"level":      {"level"},
	"visibility": {"visibility"},
	"state":      {"state"},
	"pdpScoped":  {"pdp_scoped"},
	// `org` is the REQUIRED scope, bound as a plain @org_id rather than a nullable narg.
}

// The M56 ticket-3 types. Each ships a PAIR — the instance-admin query and the visibility-scoped
// one — which must carry a byte-identical filter block, differing only by the reach/holder
// predicate. The pair is exactly where this guard earns its keep: an admin and a scoped caller
// applying the same filter must not get different rows.
var membershipFilterQueries = []struct{ module, query string }{
	{"membership", "ListMemberships"},
	{"membership", "ListMembershipsForSubject"},
	// M57 ticket 2: the dashboard candidate CTEs carry the SAME block, so a chart cannot describe a set
	// the list does not return.
	{"membership", "MembershipStats"},
	{"membership", "MembershipStatsForSubject"},
}

var membershipFacetNargs = map[string][]string{
	"unitId":        {"unit_id"},
	"personId":      {"person_id"},
	"positionId":    {"position_id"},
	"status":        {"status"},
	"effectiveFrom": {"effective_from_after", "effective_from_before"},
}

var orderFilterQueries = []struct{ module, query string }{
	{"order", "ListOrders"},
	{"order", "ListOrdersForSubject"},
	{"order", "OrderStats"},
	{"order", "OrderStatsForSubject"},
}

var orderFacetNargs = map[string][]string{
	"issuingUnitId": {"issuing_unit_id"},
	"orderTypeId":   {"order_type_id"},
	"status":        {"status"},
	"issuedOn":      {"issued_on_from", "issued_on_to"},
}

var documentFilterQueries = []struct{ module, query string }{
	{"document", "ListDocuments"},
	{"document", "ListDocumentsForSubject"},
	{"document", "DocumentStats"},
	{"document", "DocumentStatsForSubject"},
}

var documentFacetNargs = map[string][]string{
	"typeId":           {"type_id"},
	"status":           {"status"},
	"issuingCountryId": {"issuing_country_id"},
	"issuedOn":         {"issued_on_from", "issued_on_to"},
	"expiresOn":        {"expires_on_from", "expires_on_to"},
}

// The ledger ships ONE aggregate arm, not two: audit visibility is the RLS policy on audit_log rather
// than a reach predicate folded into SQL, so there is no scoped twin to hold identical. What the
// parity check still buys is the thing that would break quietly — the list and the dashboard must
// filter on the SAME nargs, or a chart would describe rows its own list does not return.
var auditFilterQueries = []struct{ module, query string }{
	{"audit", "QueryAuditLog"},
	{"audit", "AuditStats"},
}

var auditFacetNargs = map[string][]string{
	"actorType":     {"actor_type"},
	"actorPersonId": {"actor_person_id"},
	"action":        {"action"},
	"targetType":    {"target_type"},
	"targetId":      {"target_id"},
	"outcome":       {"outcome"},
	"unitId":        {"unit_id"},
	"createdAt":     {"since", "until"},
}

// orgFilterQueries: the organization facets live in the flat list plus BOTH aggregate arms. The two
// arms are the point — they differ by one gate line and must agree on everything else, or an admin
// and a scoped caller applying the same filter would be shown different worlds.
var orgFilterQueries = []struct{ module, query string }{
	{"tenant", "ListOrganizations"},
	{"tenant", "OrganizationStats"},
	{"tenant", "OrganizationStatsForSubject"},
}

var orgFacetNargs = map[string][]string{
	"domain":     {"domain_id"},
	"visibility": {"visibility"},
	"state":      {"state"},
}

// languoidFilterQueries: FOUR queries, because languoid is the second type with an R-21 search twin
// (after person). List and Search, aggregate and aggregate-Search — each pair differing only by the
// trigram line, and all four carrying one filter block.
var languoidFilterQueries = []struct{ module, query string }{
	{"language", "ListLanguoids"},
	{"language", "SearchLanguoids"},
	{"language", "LanguoidStats"},
	{"language", "LanguoidStatsSearch"},
}

var languoidFacetNargs = map[string][]string{
	"level":     {"level"},
	"family":    {"family"},
	"macroarea": {"macroarea"},
	"status":    {"status"},
}

// companyFilterQueries: SIX queries, the largest group here, because company is the first type with
// BOTH a visibility gate and an R-21 search twin — {list, search} plus the four aggregate arms
// {plain, search} × {admin, scoped}. Person has the same square but splits its scoped half across
// membership's reach queries; company's four sit in one file, which makes the drift risk higher, not
// lower: six near-identical blocks are easy to edit five of.
var companyFilterQueries = []struct{ module, query string }{
	{"company", "ListCompanies"},
	{"company", "SearchCompanies"},
	{"company", "CompanyStats"},
	{"company", "CompanyStatsSearch"},
	{"company", "CompanyStatsForSubject"},
	{"company", "CompanyStatsForSubjectSearch"},
}

var companyFacetNargs = map[string][]string{
	"legalForm":         {"legal_form_id"},
	"ownershipCategory": {"ownership_category"},
	"countryId":         {"country_id"},
	"industryClass":     {"industry_class_id"},
	"foundedOn":         {"founded_on_from", "founded_on_to"},
	"state":             {"state"},
}

// institutionFilterQueries: the same six-query square as company, one domain over.
var institutionFilterQueries = []struct{ module, query string }{
	{"education", "ListInstitutions"},
	{"education", "SearchInstitutions"},
	{"education", "InstitutionStats"},
	{"education", "InstitutionStatsSearch"},
	{"education", "InstitutionStatsForSubject"},
	{"education", "InstitutionStatsForSubjectSearch"},
}

var institutionFacetNargs = map[string][]string{
	"kindId":    {"kind_id"},
	"countryId": {"country_id"},
	"foundedOn": {"founded_on_from", "founded_on_to"},
	"state":     {"state"},
}

var nargRe = regexp.MustCompile(`sqlc\.narg\('([a-z_][a-z0-9_]*)'\)`)

func TestPersonFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "person", personFilterQueries, personFacetNargs)
}

func TestUnitFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "unit", unitFilterQueries, unitFacetNargs)
}

func TestMembershipFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "link__member_of", membershipFilterQueries, membershipFacetNargs)
}

func TestOrderFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "order", orderFilterQueries, orderFacetNargs)
}

func TestDocumentFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "document", documentFilterQueries, documentFacetNargs)
}

func TestAuditFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "audit", auditFilterQueries, auditFacetNargs)
}

func TestOrganizationFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "organization", orgFilterQueries, orgFacetNargs)
}

func TestLanguoidFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "languoid", languoidFilterQueries, languoidFacetNargs)
}

func TestCompanyFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "company", companyFilterQueries, companyFacetNargs)
}

func TestInstitutionFacetNargsAppearInEveryQuery(t *testing.T) {
	assertFacetNargParity(t, "institution", institutionFilterQueries, institutionFacetNargs)
}

// TestEveryRegisteredTypeHasANargGroup closes the hole the per-type tests above leave: a NEW facet
// block added to the catalog with no entry here would simply go unchecked, and the guard would stay
// green while the drift it exists to catch went unpoliced. Registering a type is therefore a
// two-place change, and this is the place that says so.
func TestEveryRegisteredTypeHasANargGroup(t *testing.T) {
	covered := map[string]bool{
		"person": true, "unit": true, "link__member_of": true, "order": true, "document": true,
		"audit": true, "organization": true, "languoid": true,
		"company": true, "institution": true,
	}
	// A raw-pgx module has no queries/*.sql for narg parity to read, so the SAME invariant — the list
	// and the stats path apply one predicate — is proven in rawpgx_test.go by an AST check that both
	// call one shared builder. Deferring rather than exempting: the type is still required to be
	// checked SOMEWHERE, this file just is not where.
	raw := rawPgxTypes()
	for _, o := range Default.All() {
		if !covered[o.Type] && !raw[o.Type] {
			t.Errorf("object type %q is registered in the catalog but has no narg-parity group in "+
				"sqlparity_test.go (and is not a raw-pgx type checked by rawpgx_test.go) — its filter "+
				"block is unchecked", o.Type)
		}
	}
}

func assertFacetNargParity(t *testing.T, objectType string, queries []struct{ module, query string }, want map[string][]string) {
	t.Helper()
	o, ok := Default.Get(objectType)
	if !ok {
		t.Fatalf("%s is not registered", objectType)
	}

	// Every facet must have a declared narg mapping, and every mapping a facet — otherwise adding a
	// facet without SQL (or removing one and leaving the mapping) would pass unnoticed.
	declared := map[string]bool{}
	for _, f := range o.Facets {
		declared[f.Key] = true
	}
	for key := range want {
		if !declared[key] {
			t.Errorf("%s: narg mapping for %q names no declared facet (stale entry)", objectType, key)
		}
	}
	for _, f := range o.Facets {
		if _, ok := want[f.Key]; !ok && !(objectType == "unit" && f.Key == "org") {
			t.Errorf("%s.%s has no narg mapping — add it here alongside the SQL predicate", objectType, f.Key)
		}
	}

	for _, q := range queries {
		body := queryBody(t, q.module, q.query)
		found := map[string]bool{}
		for _, m := range nargRe.FindAllStringSubmatch(body, -1) {
			found[m[1]] = true
		}
		var missing []string
		for key, nargs := range want {
			for _, n := range nargs {
				if !found[n] {
					missing = append(missing, fmt.Sprintf("%s (facet %s)", n, key))
				}
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("%s query %s is missing facet predicates: %s — the filter block must be identical "+
				"across all %d queries, or the same filter selects different rows on different paths",
				q.module, q.query, strings.Join(missing, ", "), len(queries))
		}
	}
}

// queryBody returns the text of one `-- name: X :kind` block from a module's sqlc sources.
func queryBody(t *testing.T, module, query string) string {
	t.Helper()
	dir := filepath.Join("..", "..", "internal", module, "adapters", "queries")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	marker := "-- name: " + query + " "
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		body := string(raw)
		i := strings.Index(body, marker)
		if i < 0 {
			continue
		}
		rest := body[i+len(marker):]
		// A block runs to the next `-- name:` marker, or to end of file.
		if j := strings.Index(rest, "-- name: "); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	t.Fatalf("query %q not found under %s — renamed or removed?", query, dir)
	return ""
}

// TestParityGuardIsNonVacuous: the block extractor must actually be finding text, or every
// "missing" check above would trivially pass on empty strings.
func TestParityGuardIsNonVacuous(t *testing.T) {
	var all []struct{ module, query string }
	for _, g := range [][]struct{ module, query string }{
		personFilterQueries, unitFilterQueries, orgFilterQueries,
		membershipFilterQueries, orderFilterQueries, documentFilterQueries,
		languoidFilterQueries,
	} {
		all = append(all, g...)
	}
	for _, q := range all {
		body := queryBody(t, q.module, q.query)
		if len(body) < 100 {
			t.Errorf("%s.%s extracted only %d chars — the block parser is broken", q.module, q.query, len(body))
		}
		if !strings.Contains(body, "sqlc.narg(") {
			t.Errorf("%s.%s contains no sqlc.narg at all — extraction or the query is wrong", q.module, q.query)
		}
	}
}
