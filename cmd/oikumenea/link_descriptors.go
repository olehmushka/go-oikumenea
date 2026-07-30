// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Link-descriptor wiring (review-2026-09 R-27 / D-LinkTraversal): composition glue registering every
// traversable reified link table's Descriptor on the generic link-traversal engine, plus the
// per-neighbor-type D-VisibilityScope adapter (R-30) each neighbor is trimmed through. Lives beside
// main.go because it is pure wiring; the descriptor identifiers come from the migrations and are
// validated against the drift-proof pkg/rid link-type registry (R-28) at Register time, and the
// engine's MustBeBound (main.go's boot seam loop) fails startup if any kind=link type is neither
// registered here nor explicitly exempt — so a link table added by a future milestone shows up in
// /o/[rid] and /graph without touching web/, or fails boot until wired here.
//
// Visibility follows the Phase-14 precedent (search_providers.go): person neighbors are trimmed by
// the D-PersonReadScope membership semi-join; UNIT neighbors by the shadow-visibility gate (tenant's
// own unit lists shadow-trim, so catalog scope would be wider); every other neighbor type is a
// reference/registry object whose owning module coarse-gates by a single read permission with no row
// trim — catalog scope, which is differential-equal to that behavior (R-30). Neighbor LABELS are
// resolved here too: each neighbor type registers a labeler that returns a locale→text display name
// (D-i18n) so link/graph rows show a real name, not the RID tail (the former open seam, now closed).
package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	authzapp "github.com/olegamysk/go-oikumenea/internal/authorization/application"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/scope"
	linksapp "github.com/olegamysk/go-oikumenea/internal/links/application"
	linksdomain "github.com/olegamysk/go-oikumenea/internal/links/domain"
	localizationapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	membershipapp "github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

func registerLinkDescriptors(
	linksSvc *linksapp.Service,
	pool *pgxpool.Pool,
	membershipSvc *membershipapp.Service,
	authzSvc *authzapp.Service,
	locSvc *localizationapp.Service,
) error {
	// --- visibility scopes, one per neighbor object type (D-VisibilityScope) ---
	person := scope.NewPersonScope(membershipSvc.SubjectReadablePersonsAmong)
	catalog := scope.NewCatalogScope()
	// Unit neighbors: identity owning-unit map + shadow flags read from tenant_units, then the
	// authorization shadow gate. Unmapped (deleted/absent) unit ids are dropped (fail closed).
	units := scope.NewUnitScope(
		func(ctx context.Context, ids []string) (map[string]string, map[string]bool, error) {
			unitOf := make(map[string]string, len(ids))
			shadow := make(map[string]bool, len(ids))
			rows, err := pool.Query(ctx,
				`SELECT id::text, visibility FROM oikumenea.tenant_units WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
			if err != nil {
				return nil, nil, err
			}
			defer rows.Close()
			for rows.Next() {
				var id, vis string
				if err := rows.Scan(&id, &vis); err != nil {
					return nil, nil, err
				}
				unitOf[id] = id
				shadow[id] = vis == "shadow"
			}
			return unitOf, shadow, rows.Err()
		},
		authzSvc.FilterVisibleUnits,
	)

	linksSvc.RegisterVisibility("person", person)
	linksSvc.RegisterVisibility("unit", units)
	for _, t := range []string{
		"organization", "languoid", "location", "writing_system", "rank", "building",
		"education_position", "course", "curriculum_version", "publication", "research_group",
		"governance_body", "grant", "qualification", "scholarship", "company_position",
		"industry_class", "taxon", "clergy_grade", "vehicle", "vehicle_brand", "account",
	} {
		linksSvc.RegisterVisibility(t, catalog)
	}

	// --- neighbor labelers (D-LinkTraversal, R-27 labeler seam): resolve each neighbor RID to a
	// locale→text display name (D-i18n) so link/graph rows show a real name, not the RID tail. Each
	// overlay labeler reads the neighbor's base `name`/`title` text keyed by RID, then overlays the
	// i18n translation store via localization.NamesByID (types with no translation rows degrade to a
	// single default-locale entry). person is dedicated (per-locale name variants). Types with no
	// human name (curriculum_version, vehicle, account) keep the RID-tail fallback. ---
	linksSvc.RegisterLabeler("person", personLabeler(pool, locSvc))
	for _, l := range []struct{ typ, table, col, entity string }{
		{"unit", "tenant_units", "name", "unit"},
		{"organization", "tenant_organizations", "name", "organization"},
		{"languoid", "language_languoids", "name", "languoid"},
		{"writing_system", "writing_systems", "name", "writing_system"},
		{"rank", "rank_ranks", "name", "rank"},
		{"taxon", "religion_taxa", "name", "religion_taxon"},
		{"clergy_grade", "religion_clergy_grades", "name", "clergy_grade"},
		{"location", "location_locations", "locality", "location"},
		{"building", "education_buildings", "name", "building"},
		{"education_position", "education_positions", "title", "education_position"},
		{"course", "education_courses", "title", "course"},
		{"publication", "education_publications", "title", "publication"},
		{"research_group", "education_research_groups", "name", "research_group"},
		{"governance_body", "education_governance_bodies", "name", "governance_body"},
		{"grant", "education_grants", "title", "grant"},
		{"qualification", "education_qualifications", "name", "qualification"},
		{"scholarship", "education_scholarships", "name", "scholarship"},
		{"company_position", "company_positions", "title", "company_position"},
		{"industry_class", "company_industry_classes", "name", "industry_class"},
		{"vehicle_brand", "vehicle_brands", "name", "vehicle_brand"},
	} {
		linksSvc.RegisterLabeler(l.typ, overlayLabeler(pool, locSvc, l.table, l.col, l.entity))
	}

	// --- exemptions: kind=link types that are deliberately NOT generically traversable ---
	linksSvc.Exempt(rid.SvcI18n, 1, "locale_language: the locale end is a text code, not a RID")
	linksSvc.Exempt(rid.SvcPerson, 9, "has_ethnicity: the ethnicity end is an envelope-encrypted value, not a RID")
	linksSvc.Exempt(rid.SvcPerson, 11, "party_membership: the party end is encrypted / a free external-org reference")
	linksSvc.Exempt(rid.SvcPerson, 12, "government_position: the org end is polymorphic text with no kind discriminator")
	linksSvc.Exempt(rid.SvcPerson, 13, "lobbying_rel: registrant/client ends are free text, not RIDs")
	linksSvc.Exempt(rid.SvcAuthz, 1, "has_role: a three-way assignment (role + target unit), not a 2-ended object link")
	linksSvc.Exempt(rid.SvcAuthz, 2, "instance_admin: the instance plane, no neighbor object")
	linksSvc.Exempt(rid.SvcAuthz, 3, "principal_grant: authority plumbing — the permission end is a code, not a RID, and machine authority is not object-graph data (M51)")
	linksSvc.Exempt(rid.SvcReligion, 3, "affiliated_with: multi-ended optional affiliation (taxon + tradition + community units)")

	// --- descriptors ---
	for _, d := range descriptors() {
		if err := linksSvc.Register(d); err != nil {
			return err
		}
	}
	return nil
}

// overlayLabeler builds a LabelFunc for an RID-keyed neighbor type: fetch the base display text from
// <table>.<nameCol> (keyed by the neighbor's RID id), then overlay the i18n translation store via
// localization.NamesByID → a locale→text map. A type with no translation rows yields a single-entry
// map (default locale). Raw SQL over a COMPILE-TIME table/column (never user input), Sanitize'd — the
// same justified dynamic-SQL pattern as the engine and the unit-visibility scope above.
//
// It reads through the REQUEST-PINNED connection (db.RequestQuerier), not the bare pool: several label
// tables are row-secured (membership_positions, tenant_units), and on an unpinned connection the app.*
// GUCs are unset, so an RLS-protected table returns ZERO rows and every label silently disappears.
// M57 ticket 2 found this the moment a labeler was pointed at membership_positions — the position
// buckets came back unlabelled while every non-RLS type resolved. Units were affected less visibly:
// their policy has a public-read arm, so PUBLIC units resolved and in-reach SHADOW ones did not.
func overlayLabeler(pool *pgxpool.Pool, loc *localizationapp.Service, table, nameCol, entityType string) linksapp.LabelFunc {
	q := fmt.Sprintf(`SELECT id::text, %s FROM %s WHERE id = ANY($1::uuid[])`,
		pgx.Identifier{nameCol}.Sanitize(), pgx.Identifier{"oikumenea", table}.Sanitize())
	return func(ctx context.Context, ids []string) (map[string]map[string]string, error) {
		rows, err := db.RequestQuerier(ctx, pool).Query(ctx, q, ids)
		if err != nil {
			return nil, err
		}
		base := make(map[string]string, len(ids))
		for rows.Next() {
			var id string
			var name *string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return nil, err
			}
			if name != nil && *name != "" {
				base[id] = *name
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return loc.NamesByID(ctx, entityType, base)
	}
}

// personLabeler resolves a person's display name as a locale→text map: the base display_name at the
// default locale, overlaid with the person's canonical per-locale name variants (is_primary/preferred
// only — never aliases). Person names live in the per-person variant table, not the i18n translation
// store, so this is dedicated rather than an overlayLabeler.
func personLabeler(pool *pgxpool.Pool, loc *localizationapp.Service) linksapp.LabelFunc {
	return func(ctx context.Context, ids []string) (map[string]map[string]string, error) {
		def, err := loc.DefaultLocale(ctx)
		if err != nil {
			return nil, err
		}
		out := make(map[string]map[string]string, len(ids))
		rows, err := db.RequestQuerier(ctx, pool).Query(ctx,
			`SELECT id::text, display_name FROM oikumenea.person_persons WHERE id = ANY($1::uuid[])`, ids)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var dn *string
			if err := rows.Scan(&id, &dn); err != nil {
				rows.Close()
				return nil, err
			}
			if dn != nil && *dn != "" && def != "" {
				out[id] = map[string]string{def: *dn}
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		// Canonical per-locale name forms only (variant_kind='transliteration'); aliases/akas excluded.
		// is_primary last so it wins a last-write when a locale has several transliterations.
		vrows, err := db.RequestQuerier(ctx, pool).Query(ctx,
			`SELECT person_id::text, locale, display_name FROM oikumenea.person_name_variants
			 WHERE person_id = ANY($1::uuid[]) AND variant_kind = 'transliteration' AND display_name IS NOT NULL
			 ORDER BY is_primary`, ids)
		if err != nil {
			return nil, err
		}
		defer vrows.Close()
		for vrows.Next() {
			var pid string
			var locale, dn *string
			if err := vrows.Scan(&pid, &locale, &dn); err != nil {
				return nil, err
			}
			if locale == nil || *locale == "" || dn == nil || *dn == "" {
				continue
			}
			m := out[pid]
			if m == nil {
				m = map[string]string{}
				out[pid] = m
			}
			m[*locale] = *dn
		}
		return out, vrows.Err()
	}
}

// plain is a uuid FK endpoint pointing at one object type.
func plain(col, objectType string) linksdomain.Endpoint {
	return linksdomain.Endpoint{Column: col, Targets: []linksdomain.Target{{Type: objectType}}}
}

// personOrCompany is the F-014 polymorphic holder/owner end: a text RID + a kind discriminator, a
// person (6,1,1) or a company = tenant organization (4,1,6) per M41/D-UnifiedOrgGraph.
func personOrCompany(idCol, kindCol string) linksdomain.Endpoint {
	return linksdomain.Endpoint{
		Column:  idCol,
		KindCol: kindCol,
		Targets: []linksdomain.Target{
			{KindValue: "person", Type: "person"},
			{KindValue: "company", Type: "organization"},
		},
	}
}

func descriptors() []linksdomain.Descriptor {
	p := string(authzdomain.PermPersonRead)
	return []linksdomain.Descriptor{
		// tenant
		{Service: rid.SvcTenant, Code: 1, LinkName: "parent_of", Table: "oikumenea.tenant_unit_edges",
			A: plain("parent_id", "unit"), B: plain("child_id", "unit"), Permission: string(authzdomain.PermUnitRead), NoSoftDelete: true},
		{Service: rid.SvcTenant, Code: 2, LinkName: "unit_language", Table: "oikumenea.tenant_unit_languages",
			A: plain("unit_id", "unit"), B: plain("language_id", "languoid"), Permission: string(authzdomain.PermUnitRead)},
		// language
		{Service: rid.SvcLanguage, Code: 1, LinkName: "written_in", Table: "oikumenea.language_writing_systems",
			A: plain("languoid_id", "languoid"), B: plain("writing_system_id", "writing_system"), Permission: string(authzdomain.PermLanguageRead), AttrCols: []string{"is_primary"}, NoSoftDelete: true},
		// person links. holds_rank/speaks keep the coarse person.read: they are directory attributes
		// returned INSIDE the person aggregate (getPerson), so a separate arm code would hide them in the
		// graph while the person page still shows them — incoherent (D-LinkPermissions). The relationship
		// graph + address each carry their OWN code, the same one their dedicated list endpoint requires.
		{Service: rid.SvcPerson, Code: 1, LinkName: "holds_rank", Table: "oikumenea.person_ranks",
			A: plain("person_id", "person"), B: plain("rank_id", "rank"), Permission: p},
		{Service: rid.SvcPerson, Code: 2, LinkName: "partnered_with", Table: "oikumenea.person_partnerships",
			A: plain("person_id_a", "person"), B: plain("person_id_b", "person"), Permission: string(authzdomain.PermPersonPartnershipRead), AttrCols: []string{"status"}},
		{Service: rid.SvcPerson, Code: 3, LinkName: "kin_parent_of", Table: "oikumenea.person_kinships",
			A: plain("parent_id", "person"), B: plain("child_id", "person"), Permission: string(authzdomain.PermPersonKinshipRead), AttrCols: []string{"status"}},
		{Service: rid.SvcPerson, Code: 4, LinkName: "guardian_of", Table: "oikumenea.person_guardianships",
			A: plain("guardian_id", "person"), B: plain("ward_id", "person"), Permission: string(authzdomain.PermPersonGuardianshipRead), AttrCols: []string{"status"}},
		{Service: rid.SvcPerson, Code: 5, LinkName: "sponsor_of", Table: "oikumenea.person_sponsorships",
			A: plain("sponsor_id", "person"), B: plain("sponsored_id", "person"), Permission: string(authzdomain.PermPersonSponsorshipRead), AttrCols: []string{"status"}},
		{Service: rid.SvcPerson, Code: 6, LinkName: "next_of_kin", Table: "oikumenea.person_next_of_kin",
			A: plain("subject_id", "person"), B: plain("contact_id", "person"), Permission: string(authzdomain.PermPersonNextOfKinRead), AttrCols: []string{"status"}},
		{Service: rid.SvcPerson, Code: 7, LinkName: "associated_with", Table: "oikumenea.person_associations",
			A: plain("person_id_a", "person"), B: plain("person_id_b", "person"), Permission: string(authzdomain.PermPersonAssociationRead), AttrCols: []string{"status"}},
		{Service: rid.SvcPerson, Code: 8, LinkName: "speaks", Table: "oikumenea.person_languages",
			A: plain("person_id", "person"), B: plain("language_id", "languoid"), Permission: p},
		{Service: rid.SvcPerson, Code: 10, LinkName: "lives_at", Table: "oikumenea.person_addresses",
			A: plain("person_id", "person"), B: plain("location_id", "location"), Permission: string(authzdomain.PermPersonAddressRead), AttrCols: []string{"role"}},
		// membership
		// FilterCol status='active' both restricts the graph to CURRENT memberships and matches the
		// membership_memberships partial indexes (…WHERE status='active' AND deleted_at IS NULL), so
		// traversal — including a unit's members at M49 scale — stays index-backed instead of seq-scanning.
		{Service: rid.SvcMembership, Code: 1, LinkName: "member_of", Table: "oikumenea.membership_memberships",
			A: plain("person_id", "person"), B: plain("unit_id", "unit"), Permission: string(authzdomain.PermMembershipRead),
			AttrCols: []string{"status"}, FilterCol: "status", FilterVal: "active"},
		// education
		{Service: rid.SvcEducation, Code: 2, LinkName: "studied_at", Table: "oikumenea.person_education_enrollments",
			A: plain("person_id", "person"), B: plain("institution_id", "organization"), Permission: string(authzdomain.PermEducationRead)},
		{Service: rid.SvcEducation, Code: 3, LinkName: "resided_in_dormitory", Table: "oikumenea.person_dormitory_stays",
			A: plain("person_id", "person"), B: plain("building_id", "building"), Permission: string(authzdomain.PermEducationRead)},
		{Service: rid.SvcEducation, Code: 4, LinkName: "holds_education_position", Table: "oikumenea.education_appointments",
			A: plain("person_id", "person"), B: plain("position_id", "education_position"), Permission: string(authzdomain.PermEducationRead)},
		{Service: rid.SvcEducation, Code: 5, LinkName: "curriculum_item", Table: "oikumenea.education_curriculum_items",
			A: plain("version_id", "curriculum_version"), B: plain("course_id", "course"), Permission: string(authzdomain.PermEducationRead)},
		{Service: rid.SvcEducation, Code: 6, LinkName: "course_prerequisite", Table: "oikumenea.education_course_prerequisites",
			A: plain("course_id", "course"), B: plain("required_course_id", "course"), Permission: string(authzdomain.PermEducationRead)},
		{Service: rid.SvcEducation, Code: 7, LinkName: "authored_publication", Table: "oikumenea.person_publication_authorships",
			A: plain("person_id", "person"), B: plain("publication_id", "publication"), Permission: string(authzdomain.PermEducationRead), AttrCols: []string{"author_order"}},
		{Service: rid.SvcEducation, Code: 8, LinkName: "member_of_research_group", Table: "oikumenea.person_research_memberships",
			A: plain("person_id", "person"), B: plain("group_id", "research_group"), Permission: string(authzdomain.PermEducationRead)},
		{Service: rid.SvcEducation, Code: 9, LinkName: "holds_grant", Table: "oikumenea.person_grant_holdings",
			A: plain("person_id", "person"), B: plain("grant_id", "grant"), Permission: string(authzdomain.PermEducationRead), AttrCols: []string{"role", "status"}},
		{Service: rid.SvcEducation, Code: 10, LinkName: "member_of_governance_body", Table: "oikumenea.person_governance_memberships",
			A: plain("person_id", "person"), B: plain("body_id", "governance_body"), Permission: string(authzdomain.PermEducationRead)},
		{Service: rid.SvcEducation, Code: 11, LinkName: "awarded_qualification", Table: "oikumenea.person_education_qualifications",
			A: plain("person_id", "person"), B: plain("qualification_id", "qualification"), Permission: string(authzdomain.PermEducationRead), AttrCols: []string{"status"}},
		{Service: rid.SvcEducation, Code: 12, LinkName: "awarded_scholarship", Table: "oikumenea.person_scholarship_awards",
			A: plain("person_id", "person"), B: plain("scholarship_id", "scholarship"), Permission: string(authzdomain.PermEducationRead)},
		// company
		{Service: rid.SvcCompany, Code: 1, LinkName: "holds_company_position", Table: "oikumenea.company_appointments",
			A: plain("person_id", "person"), B: plain("position_id", "company_position"), Permission: string(authzdomain.PermCompanyRead)},
		{Service: rid.SvcCompany, Code: 2, LinkName: "founded", Table: "oikumenea.company_foundings",
			A: plain("company_id", "organization"), B: personOrCompany("holder_id", "holder_kind"), Permission: string(authzdomain.PermCompanyRead)},
		{Service: rid.SvcCompany, Code: 3, LinkName: "owns_stake", Table: "oikumenea.company_shareholdings",
			A: plain("company_id", "organization"), B: personOrCompany("holder_id", "holder_kind"), Permission: string(authzdomain.PermCompanyRead)},
		{Service: rid.SvcCompany, Code: 4, LinkName: "beneficiary_of", Table: "oikumenea.company_beneficiaries",
			A: plain("company_id", "organization"), B: plain("person_id", "person"), Permission: string(authzdomain.PermCompanyRead)},
		{Service: rid.SvcCompany, Code: 5, LinkName: "succeeded_by", Table: "oikumenea.company_successions",
			A: plain("predecessor_id", "organization"), B: plain("successor_id", "organization"), Permission: string(authzdomain.PermCompanyRead), AttrCols: []string{"kind"}},
		{Service: rid.SvcCompany, Code: 6, LinkName: "branch_of", Table: "oikumenea.company_branches",
			A: plain("branch_id", "organization"), B: plain("parent_id", "organization"), Permission: string(authzdomain.PermCompanyRead)},
		{Service: rid.SvcCompany, Code: 7, LinkName: "has_industry", Table: "oikumenea.company_industry_assignments",
			A: plain("company_id", "organization"), B: plain("industry_class_id", "industry_class"), Permission: string(authzdomain.PermCompanyRead)},
		{Service: rid.SvcCompany, Code: 8, LinkName: "located_at", Table: "oikumenea.company_locations",
			A: plain("company_id", "organization"), B: plain("location_id", "location"), Permission: string(authzdomain.PermCompanyRead)},
		// religion
		{Service: rid.SvcReligion, Code: 1, LinkName: "classified_as", Table: "oikumenea.religion_org_classifications",
			A: plain("unit_id", "unit"), B: plain("taxon_id", "taxon"), Permission: string(authzdomain.PermReligionRead)},
		{Service: rid.SvcReligion, Code: 2, LinkName: "clergy_credential", Table: "oikumenea.religion_clergy_credentials",
			A: plain("person_id", "person"), B: plain("clergy_grade_id", "clergy_grade"), Permission: string(authzdomain.PermReligionRead)},
		{Service: rid.SvcReligion, Code: 4, LinkName: "site_of", Table: "oikumenea.religion_sites",
			A: plain("org_unit_id", "unit"), B: plain("location_id", "location"), Permission: string(authzdomain.PermReligionRead)},
		// vehicle
		{Service: rid.SvcVehicle, Code: 1, LinkName: "manufactured_by", Table: "oikumenea.vehicle_brand_manufacturers",
			A: plain("brand_id", "vehicle_brand"), B: plain("company_id", "organization"), Permission: string(authzdomain.PermVehicleRead)},
		{Service: rid.SvcVehicle, Code: 2, LinkName: "registered_to", Table: "oikumenea.vehicle_registrations",
			A: plain("vehicle_id", "vehicle"), B: personOrCompany("owner_id", "owner_kind"), Permission: string(authzdomain.PermVehicleRegistrationRead)},
		// finance
		{Service: rid.SvcFinance, Code: 1, LinkName: "held_by", Table: "oikumenea.finance_account_holders",
			A: plain("account_id", "account"), B: personOrCompany("holder_id", "holder_kind"), Permission: string(authzdomain.PermFinanceHolderRead)},
	}
}
