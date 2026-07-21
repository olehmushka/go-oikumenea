package main

// Drift guard for D-Temporal (review-2026-09 R-31): every reified Link type (kind=2 in pkg/rid) must
// carry an explicit history tier, and every Link declared to carry native validity must actually have
// a validity column in its migration DDL. This is the temporal analogue of the R-27 link-coverage
// assertion and the R-28 RID coherence check — a new milestone's Link fails this test until its tier
// is declared here (and, if tier-a, its table grows a valid_from/effective_from/… column), so the
// ontology's temporal boundary can never silently drift the way §4.1 warned it had.
//
// Tiers (D-Temporal): tierValidity = the Link's truth-interval is dated on the row itself (valid_from
// /valid_to, or the grandfathered effective_from/to · granted_at/revoked_at · founded_on · awarded_on
// equivalents); tierExempt = a reference/structural association whose change is a correction, not a
// dated historical event (validity would be noise). Object history (the review's tier b) is served
// separately by AuditService.getObjectHistory over the audit ledger and needs no per-Link column.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

type temporalTier int

const (
	tierValidity temporalTier = iota // (a) native validity dating on the link row
	tierExempt                       // (c) reference/structural — history-exempt by decision
)

// temporalTiers is the machine form of D-Temporal's tier table: the authoritative classification of
// every kind=Link RID type. Keyed by (service, typeCode). Keep in lockstep with the decisions.md
// D-Temporal tier table (the doc is the human-readable mirror of this map).
var temporalTiers = map[[2]int]temporalTier{
	// --- tier (c): history-exempt reference / structural associations ---
	{rid.SvcI18n, 1}:      tierExempt, // locale_language — a locale's languages (reference association)
	{rid.SvcTenant, 2}:    tierExempt, // unit_language — a unit's working languages (reference association)
	{rid.SvcPerson, 9}:    tierExempt, // has_ethnicity — encrypted self-declared attribute, not a dated edge
	{rid.SvcLanguage, 1}:  tierExempt, // written_in — languoid↔script Glottolog linguistic fact
	{rid.SvcEducation, 5}: tierExempt, // curriculum_item — structural within a versioned curriculum snapshot
	{rid.SvcEducation, 6}: tierExempt, // course_prerequisite — structural academic fact

	// --- tier (a): native validity dating ---
	{rid.SvcTenant, 1}:     tierValidity, // parent_of (valid_from/to)
	{rid.SvcPerson, 1}:     tierValidity, // holds_rank (valid_from/to)
	{rid.SvcPerson, 2}:     tierValidity, // partnered_with (effective_from/to)
	{rid.SvcPerson, 3}:     tierValidity, // kin_parent_of (valid_from/to)
	{rid.SvcPerson, 4}:     tierValidity, // guardian_of (effective_from/to)
	{rid.SvcPerson, 5}:     tierValidity, // sponsor_of (effective_from/to)
	{rid.SvcPerson, 6}:     tierValidity, // next_of_kin (valid_from/to)
	{rid.SvcPerson, 7}:     tierValidity, // associated_with (valid_from/to)
	{rid.SvcPerson, 8}:     tierValidity, // speaks (valid_from/to)
	{rid.SvcPerson, 10}:    tierValidity, // lives_at (valid_from/to)
	{rid.SvcPerson, 11}:    tierValidity, // party_membership (valid_from/to)
	{rid.SvcPerson, 12}:    tierValidity, // government_position (valid_from/to)
	{rid.SvcPerson, 13}:    tierValidity, // lobbying_rel (valid_from/to)
	{rid.SvcMembership, 1}: tierValidity, // member_of (effective_from/to)
	{rid.SvcAuthz, 1}:      tierValidity, // has_role (granted_at/revoked_at/expires_at)
	{rid.SvcAuthz, 2}:      tierValidity, // instance_admin (granted_at/revoked_at)
	{rid.SvcAuthz, 3}:      tierValidity, // principal_grant (granted_at/revoked_at) — M51
	{rid.SvcEducation, 2}:  tierValidity, // studied_at (effective_from/to)
	{rid.SvcEducation, 3}:  tierValidity, // resided_in_dormitory (effective_from/to)
	{rid.SvcEducation, 4}:  tierValidity, // holds_education_position (effective_from/to)
	{rid.SvcEducation, 7}:  tierValidity, // authored_publication (effective_from/to)
	{rid.SvcEducation, 8}:  tierValidity, // member_of_research_group (effective_from/to)
	{rid.SvcEducation, 9}:  tierValidity, // holds_grant (effective_from/to)
	{rid.SvcEducation, 10}: tierValidity, // member_of_governance_body (effective_from/to)
	{rid.SvcEducation, 11}: tierValidity, // awarded_qualification (awarded_on)
	{rid.SvcEducation, 12}: tierValidity, // awarded_scholarship (effective_from/to)
	{rid.SvcCompany, 1}:    tierValidity, // holds_company_position (effective_from/to)
	{rid.SvcCompany, 2}:    tierValidity, // founded (founded_on)
	{rid.SvcCompany, 3}:    tierValidity, // owns_stake (effective_from/to)
	{rid.SvcCompany, 4}:    tierValidity, // beneficiary_of (valid_from/to)
	{rid.SvcCompany, 5}:    tierValidity, // succeeded_by (valid_from/to)
	{rid.SvcCompany, 6}:    tierValidity, // branch_of (valid_from/to)
	{rid.SvcCompany, 7}:    tierValidity, // has_industry (valid_from/to)
	{rid.SvcCompany, 8}:    tierValidity, // located_at (valid_from/to)
	{rid.SvcReligion, 1}:   tierValidity, // classified_as (valid_from/to)
	{rid.SvcReligion, 2}:   tierValidity, // clergy_credential (effective_from/to)
	{rid.SvcReligion, 3}:   tierValidity, // affiliated_with (effective_from/to)
	{rid.SvcReligion, 4}:   tierValidity, // site_of (valid_from/to)
	{rid.SvcVehicle, 1}:    tierValidity, // manufactured_by (effective_from/to)
	{rid.SvcVehicle, 2}:    tierValidity, // registered_to (effective_from/to)
	{rid.SvcFinance, 1}:    tierValidity, // held_by (effective_from/to)
}

// linkTables maps every kind=Link type to its backing table, for the schema-backing check. The
// traversable links come from the live descriptor registry (single source of truth); the
// traversal-exempt kind=Link tables are listed here explicitly (they own no descriptor).
func linkTables() map[[2]int]string {
	m := map[[2]int]string{}
	for _, d := range descriptors() {
		m[[2]int{d.Service, d.Code}] = d.Table
	}
	// traversal-exempt kind=Link tables (not RID→RID descriptors; declared in link_descriptors.go's
	// Exempt calls). Only the tier-a ones need a validity column; the rest are harmlessly present.
	for k, tbl := range map[[2]int]string{
		{rid.SvcI18n, 1}:     "oikumenea.i18n_locale_languages",
		{rid.SvcPerson, 9}:   "oikumenea.person_ethnicities",
		{rid.SvcPerson, 11}:  "oikumenea.person_party_memberships",
		{rid.SvcPerson, 12}:  "oikumenea.person_government_positions",
		{rid.SvcPerson, 13}:  "oikumenea.person_lobbying_relationships",
		{rid.SvcAuthz, 1}:    "oikumenea.authz_role_assignments",
		{rid.SvcAuthz, 2}:    "oikumenea.authz_instance_admins",
		{rid.SvcAuthz, 3}:    "oikumenea.authz_principal_grants",
		{rid.SvcReligion, 3}: "oikumenea.religion_affiliations",
	} {
		m[k] = tbl
	}
	return m
}

// linkKeys returns every kind=Link (service, typeCode) from the drift-proof pkg/rid registry.
func linkKeys() [][2]int {
	var out [][2]int
	for _, t := range rid.Types() {
		if t.Kind == int(rid.KindLink) {
			out = append(out, [2]int{t.Service, t.Code})
		}
	}
	return out
}

// TestTemporalTierCoverage: every kind=Link type is classified, and there are no stale classifications
// (a Link removed from pkg/rid must be removed here too). This is the drift guard R-31 makes executable.
func TestTemporalTierCoverage(t *testing.T) {
	keys := linkKeys()
	for _, k := range keys {
		if _, ok := temporalTiers[k]; !ok {
			t.Errorf("kind=Link type %v has no D-Temporal tier — classify it in temporalTiers (tierValidity or tierExempt)", k)
		}
	}
	live := map[[2]int]bool{}
	for _, k := range keys {
		live[k] = true
	}
	for k := range temporalTiers {
		if !live[k] {
			t.Errorf("temporalTiers has stale entry %v — no such kind=Link type in pkg/rid", k)
		}
	}
}

// TestValidityLinksHaveDatingColumn: a Link declared tier-a must actually carry a validity column in
// its migration DDL — so "this link is dated" can never be an unbacked claim (the R-31 acceptance:
// the mandate is real, not documentation).
func TestValidityLinksHaveDatingColumn(t *testing.T) {
	migDir := filepath.Join("..", "..", "migrations")
	all := readAllMigrations(t, migDir)
	tables := linkTables()
	// Accepted validity representations (D-Temporal): the canonical pair + the grandfathered aliases.
	datingCol := regexp.MustCompile(`\b(valid_from|effective_from|granted_at|founded_on|awarded_on)\b`)

	for k, tier := range temporalTiers {
		if tier != tierValidity {
			continue
		}
		table := tables[k]
		if table == "" {
			t.Errorf("tier-a link %v has no known table for the schema-backing check", k)
			continue
		}
		ddl := createTableBlock(all, table)
		if ddl == "" {
			t.Errorf("tier-a link %v: could not find CREATE TABLE %s in migrations", k, table)
			continue
		}
		if !datingCol.MatchString(ddl) {
			t.Errorf("tier-a link %v (%s) is declared to carry native validity but its DDL has no valid_from/effective_from/granted_at/founded_on/awarded_on column", k, table)
		}
	}
}

func readAllMigrations(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		b.Write(body)
		b.WriteString("\n")
	}
	return b.String()
}

// createTableBlock returns the text of `CREATE TABLE <table> ( … );` (through the first line-anchored
// `);`), or "" if absent. Any ALTER TABLE … ADD COLUMN for the same table is also appended, so a
// validity column added by a later ALTER (rather than inline in CREATE TABLE) still counts.
func createTableBlock(all, table string) string {
	start := strings.Index(all, "CREATE TABLE "+table+" ")
	if start < 0 {
		return ""
	}
	rest := all[start:]
	end := strings.Index(rest, "\n);")
	if end < 0 {
		return ""
	}
	block := rest[:end]
	// Fold in any ALTER TABLE <table> … ADD COLUMN for the same table (defensive — the current tier-a
	// columns are inline in CREATE TABLE, but a future Link may add validity via ALTER). Only the parts
	// AFTER an occurrence of "ALTER TABLE <table>" belong to this table — index 0 is the preamble before
	// the first such ALTER and must be skipped so other tables' DDL never leaks in.
	parts := strings.Split(all, "ALTER TABLE "+table+"\n")
	for _, stmt := range parts[1:] {
		if semi := strings.Index(stmt, ";"); semi >= 0 && strings.Contains(stmt[:semi], "ADD COLUMN") {
			block += "\n" + stmt[:semi]
		}
	}
	return block
}
