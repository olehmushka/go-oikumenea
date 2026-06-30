package rid

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// The RID registries — the Go mirror of oikumenea.platform_rid_services / platform_rid_types
// (migration 0000). AssertMatches verifies the two agree at boot so they cannot drift. The
// authoritative *list* of types is docs/ontology-mapping.md; the numeric codes are assigned here and
// in the migration together.

var serviceNames = map[int]string{
	SvcPlatform:    "platform",
	SvcI18n:        "i18n",
	SvcAudit:       "audit",
	SvcTenant:      "tenant",
	SvcRank:        "rank",
	SvcPerson:      "person",
	SvcMembership:  "membership",
	SvcAuthz:       "authz",
	SvcAccount:     "account",
	SvcDocument:    "document",
	SvcOrder:       "order",
	SvcLocation:    "location",
	SvcLanguage:    "language",
	SvcEducation:   "education",
	SvcCompany:     "company",
	SvcReligion:    "religion",
	SvcVehicle:     "vehicle",
	SvcExternalOrg: "external_organization",
}

type typeKey struct {
	service int
	kind    int
	code    int
}

// typeNames maps (service, kind, type_code) -> type name. Mirrors the platform_rid_types seed.
var typeNames = map[typeKey]string{
	// i18n
	{SvcI18n, int(KindObject), 1}: "translation",
	// tenant
	{SvcTenant, int(KindObject), 1}: "unit",
	{SvcTenant, int(KindObject), 2}: "graph",
	{SvcTenant, int(KindObject), 3}: "unit_lifecycle_event",
	{SvcTenant, int(KindObject), 4}: "unit_code_event",
	// tenant — domains/organizations/unit-kinds (M40 / D-TenantOrganizations)
	{SvcTenant, int(KindObject), 5}: "domain",
	{SvcTenant, int(KindObject), 6}: "organization",
	{SvcTenant, int(KindObject), 7}: "unit_kind",
	{SvcTenant, int(KindObject), 8}: "org_lifecycle_event",
	{SvcTenant, int(KindLink), 1}:   "parent_of",
	// rank
	{SvcRank, int(KindObject), 1}: "system",
	{SvcRank, int(KindObject), 2}: "category",
	{SvcRank, int(KindObject), 3}: "type",
	{SvcRank, int(KindObject), 4}: "rank",
	// person objects
	{SvcPerson, int(KindObject), 1}:  "person",
	{SvcPerson, int(KindObject), 2}:  "name_variant",
	{SvcPerson, int(KindObject), 3}:  "citizenship",
	{SvcPerson, int(KindObject), 4}:  "residence",
	{SvcPerson, int(KindObject), 5}:  "email",
	{SvcPerson, int(KindObject), 6}:  "phone",
	{SvcPerson, int(KindObject), 7}:  "call_sign",
	{SvcPerson, int(KindObject), 8}:  "messenger_link",
	{SvcPerson, int(KindObject), 9}:  "social_account",
	{SvcPerson, int(KindObject), 10}: "social_handle",
	// person links
	{SvcPerson, int(KindLink), 1}: "holds_rank",
	{SvcPerson, int(KindLink), 2}: "partnered_with",
	{SvcPerson, int(KindLink), 3}: "kin_parent_of",
	{SvcPerson, int(KindLink), 4}: "guardian_of",
	{SvcPerson, int(KindLink), 5}: "sponsor_of",
	{SvcPerson, int(KindLink), 6}: "next_of_kin",
	{SvcPerson, int(KindLink), 7}: "associated_with",
	{SvcPerson, int(KindLink), 8}: "speaks",
	// membership
	{SvcMembership, int(KindObject), 1}: "position",
	{SvcMembership, int(KindLink), 1}:   "member_of",
	// authz
	{SvcAuthz, int(KindObject), 1}: "role",
	{SvcAuthz, int(KindLink), 1}:   "has_role",
	{SvcAuthz, int(KindLink), 2}:   "instance_admin",
	// account
	{SvcAccount, int(KindObject), 1}: "account",
	{SvcAccount, int(KindObject), 2}: "external_identity",
	// document
	{SvcDocument, int(KindObject), 1}: "document_type",
	{SvcDocument, int(KindObject), 2}: "document",
	{SvcDocument, int(KindObject), 3}: "personal_code",
	// order
	{SvcOrder, int(KindObject), 1}: "order_type",
	{SvcOrder, int(KindObject), 2}: "order",
	{SvcOrder, int(KindObject), 3}: "order_item",
	// location
	{SvcLocation, int(KindObject), 1}: "country",
	{SvcLocation, int(KindObject), 2}: "geo_place",
	{SvcLocation, int(KindObject), 3}: "location",
	{SvcLocation, int(KindObject), 4}: "location_type",
	// language (M18 / D-Languages)
	{SvcLanguage, int(KindObject), 1}: "languoid",
	{SvcLanguage, int(KindObject), 2}: "writing_system",
	{SvcLanguage, int(KindObject), 3}: "script_type",
	{SvcLanguage, int(KindLink), 1}:   "written_in",
	// cross-module language links (M18)
	{SvcTenant, int(KindLink), 2}: "unit_language",
	{SvcI18n, int(KindLink), 1}:   "locale_language",
	// education (M20 / D-Education; unified onto the tenant org-graph — M41 / D-UnifiedOrgGraph: an
	// institution is a tenant organization, a unit is a tenant unit, unit kinds are tenant_unit_kinds, and
	// the parent edge is a tenant graph edge — so types 1/2/7 and link 1 are GONE here).
	{SvcEducation, int(KindObject), 3}: "building",
	{SvcEducation, int(KindObject), 4}: "group",
	{SvcEducation, int(KindObject), 5}: "education_position",
	{SvcEducation, int(KindObject), 6}: "institution_kind",
	{SvcEducation, int(KindObject), 8}: "degree_level",
	{SvcEducation, int(KindLink), 2}:   "studied_at",
	{SvcEducation, int(KindLink), 3}:   "resided_in_dormitory",
	{SvcEducation, int(KindLink), 4}:   "holds_education_position",
	// education reference layer (M20 extension — university_ontology.md adoption / D-Education)
	{SvcEducation, int(KindObject), 9}:  "program",
	{SvcEducation, int(KindObject), 10}: "course",
	{SvcEducation, int(KindObject), 11}: "curriculum_version",
	{SvcEducation, int(KindObject), 12}: "research_centre",
	{SvcEducation, int(KindObject), 13}: "research_group",
	{SvcEducation, int(KindObject), 14}: "grant",
	{SvcEducation, int(KindObject), 15}: "publication",
	{SvcEducation, int(KindObject), 16}: "governance_body",
	{SvcEducation, int(KindObject), 17}: "policy",
	{SvcEducation, int(KindObject), 18}: "qualification",
	{SvcEducation, int(KindObject), 19}: "scholarship",
	{SvcEducation, int(KindObject), 20}: "accreditation_event",
	{SvcEducation, int(KindLink), 5}:    "curriculum_item",
	{SvcEducation, int(KindLink), 6}:    "course_prerequisite",
	{SvcEducation, int(KindLink), 7}:    "authored_publication",
	{SvcEducation, int(KindLink), 8}:    "member_of_research_group",
	{SvcEducation, int(KindLink), 9}:    "holds_grant",
	{SvcEducation, int(KindLink), 10}:   "member_of_governance_body",
	{SvcEducation, int(KindLink), 11}:   "awarded_qualification",
	{SvcEducation, int(KindLink), 12}:   "awarded_scholarship",
	// company (M21 / D-Companies). M41 / D-UnifiedOrgGraph: a company is a `company`-domain tenant
	// organization (no own `company` object RID); company_org_profiles is the keyed sidecar.
	{SvcCompany, int(KindObject), 2}: "legal_form",
	{SvcCompany, int(KindObject), 3}: "registration_scheme",
	{SvcCompany, int(KindObject), 4}: "industry_class",
	{SvcCompany, int(KindObject), 5}: "company_position",
	{SvcCompany, int(KindObject), 6}: "registration",
	{SvcCompany, int(KindLink), 1}:   "holds_company_position",
	{SvcCompany, int(KindLink), 2}:   "founded",
	{SvcCompany, int(KindLink), 3}:   "owns_stake",
	{SvcCompany, int(KindLink), 4}:   "beneficiary_of",
	{SvcCompany, int(KindLink), 5}:   "succeeded_by",
	{SvcCompany, int(KindLink), 6}:   "branch_of",
	{SvcCompany, int(KindLink), 7}:   "has_industry",
	{SvcCompany, int(KindLink), 8}:   "located_at",
	// religion (M22 / D-Religion) — taxonomy + organization slice
	{SvcReligion, int(KindObject), 1}: "taxon",
	{SvcReligion, int(KindObject), 2}: "taxon_rank",
	{SvcReligion, int(KindObject), 3}: "classification",
	{SvcReligion, int(KindObject), 4}: "org_kind",
	{SvcReligion, int(KindObject), 5}: "policy_kind",
	{SvcReligion, int(KindObject), 6}: "org_policy",
	// clergy (M23) + lay affiliation (M24) catalogs + reified person↔religion links
	{SvcReligion, int(KindObject), 7}:  "grade_category",
	{SvcReligion, int(KindObject), 8}:  "clergy_grade",
	{SvcReligion, int(KindObject), 9}:  "office_type",
	{SvcReligion, int(KindObject), 10}: "affiliation_type",
	// discovery (M25) — site/service-type catalogs + per-site schedules + search-only aliases
	{SvcReligion, int(KindObject), 11}: "site_type",
	{SvcReligion, int(KindObject), 12}: "service_type",
	{SvcReligion, int(KindObject), 13}: "service_schedule",
	{SvcReligion, int(KindObject), 14}: "alias",
	{SvcReligion, int(KindLink), 1}:    "classified_as",
	{SvcReligion, int(KindLink), 2}:    "clergy_credential",
	{SvcReligion, int(KindLink), 3}:    "affiliated_with",
	{SvcReligion, int(KindLink), 4}:    "site_of",
	// vehicle (M26 / D-Vehicles) — brand/model/type catalogs + the vehicle object + ownership links
	{SvcVehicle, int(KindObject), 1}: "vehicle",
	{SvcVehicle, int(KindObject), 2}: "vehicle_type",
	{SvcVehicle, int(KindObject), 3}: "vehicle_brand",
	{SvcVehicle, int(KindObject), 4}: "vehicle_model",
	{SvcVehicle, int(KindObject), 5}: "registration_number_type",
	{SvcVehicle, int(KindLink), 1}:   "manufactured_by",
	{SvcVehicle, int(KindLink), 2}:   "registered_to",
	// external-organizations (M30 / D-ExternalOrgs) — the external-org node-space + its kind catalog
	{SvcExternalOrg, int(KindObject), 1}: "external_organization",
	{SvcExternalOrg, int(KindObject), 2}: "external_org_kind",
}

// Bare person link-type names (the dispatch tokens), derived from the registry above.
const (
	LinkHoldsRank    = "holds_rank"
	LinkPartnership  = "partnered_with"
	LinkKinship      = "kin_parent_of"
	LinkGuardianship = "guardian_of"
	LinkSponsorship  = "sponsor_of"
	LinkNextOfKin    = "next_of_kin"
	LinkAssociation  = "associated_with"
)

// Querier is the minimal pgx surface AssertMatches needs (satisfied by *pgxpool.Pool / pgx.Conn / tx).
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// AssertMatches verifies the Go registries equal the seeded SQL registries (services + types), so a
// drift between migration 0000 and this package fails fast at boot rather than minting wrong RIDs.
func AssertMatches(ctx context.Context, q Querier) error {
	// services
	srvRows, err := q.Query(ctx, "SELECT code, module FROM oikumenea.platform_rid_services")
	if err != nil {
		return fmt.Errorf("rid: load services: %w", err)
	}
	dbSvc := map[int]string{}
	for srvRows.Next() {
		var code int
		var name string
		if err := srvRows.Scan(&code, &name); err != nil {
			srvRows.Close()
			return fmt.Errorf("rid: scan service: %w", err)
		}
		dbSvc[code] = name
	}
	srvRows.Close()
	if err := srvRows.Err(); err != nil {
		return fmt.Errorf("rid: services rows: %w", err)
	}
	if len(dbSvc) != len(serviceNames) {
		return fmt.Errorf("rid: service registry size mismatch: db=%d go=%d", len(dbSvc), len(serviceNames))
	}
	for code, name := range dbSvc {
		if serviceNames[code] != name {
			return fmt.Errorf("rid: service %d = %q in db but %q in go", code, name, serviceNames[code])
		}
	}
	// types
	typRows, err := q.Query(ctx, "SELECT service_code, kind, type_code, type_name FROM oikumenea.platform_rid_types WHERE kind <> 3")
	if err != nil {
		return fmt.Errorf("rid: load types: %w", err)
	}
	count := 0
	for typRows.Next() {
		var svc, kind, code int
		var name string
		if err := typRows.Scan(&svc, &kind, &code, &name); err != nil {
			typRows.Close()
			return fmt.Errorf("rid: scan type: %w", err)
		}
		if got := typeNames[typeKey{service: svc, kind: kind, code: code}]; got != name {
			typRows.Close()
			return fmt.Errorf("rid: type (%d,%d,%d) = %q in db but %q in go", svc, kind, code, name, got)
		}
		count++
	}
	typRows.Close()
	if err := typRows.Err(); err != nil {
		return fmt.Errorf("rid: types rows: %w", err)
	}
	if count != len(typeNames) {
		return fmt.Errorf("rid: type registry size mismatch: db=%d go=%d", count, len(typeNames))
	}
	return nil
}
