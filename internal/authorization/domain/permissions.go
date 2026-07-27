// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the authorization module's pure logic: the code-defined permission catalog,
// the seeded base-role definitions, the RBAC entities (Role, Assignment, InstanceAdmin), and the PDP
// engine (overview.md layering). No I/O, no framework imports — only the standard library.
//
// This file is the CATALOG: the closed vocabulary of atomic permissions and the four seeded base
// roles, both code-defined (D-Ontology's ratified divergence — an atomic permission is code, not a
// row; the authorization surface is always visible in a diff). A write to authz_role_permissions
// with a code outside Catalog() is rejected by the application; the DB stores only the membership.
package domain

// Permission is a code-defined atomic permission string — the degenerate D-Code case (the permission
// string IS its code). The closed set lives here; adding one is a code change.
type Permission string

// The atomic permission catalog (docs/modules/authorization.md). Grouped by resource. The `*.read`
// family is what the shadow gate consults on read paths. Permissions marked instance-scope below
// are only meaningful on the instance-admin plane (D-InstanceAdmin); the rest are unit-scoped.
const (
	// unit
	PermUnitRead      Permission = "unit.read"
	PermUnitCreate    Permission = "unit.create"
	PermUnitUpdate    Permission = "unit.update"
	PermUnitRecode    Permission = "unit.recode" // audited set/correct/clear of a unit's code (D-UnitCodeLifecycle)
	PermUnitLifecycle Permission = "unit.lifecycle"

	// unit edges — per graph (D-EdgePerms) + a broad fallback covering all graphs incl. custom.
	PermUnitEdgesManage            Permission = "unit.edges.manage"
	PermUnitEdgesCommandManage     Permission = "unit.edges.command.manage"
	PermUnitEdgesOperationalManage Permission = "unit.edges.operational.manage"

	// domains / unit-kinds / organizations (D-TenantOrganizations, M40). Reads are reference reads;
	// organization create/update are manager-tier; organization lifecycle is admin-tier; the domain &
	// unit-kind catalogs are managed on the instance plane (domain.manage / unit-kind.manage, below).
	// Domain/organization/kind are DIRECTORY attributes — never PDP inputs.
	PermDomainRead            Permission = "domain.read"
	PermUnitKindRead          Permission = "unit-kind.read"
	PermOrganizationRead      Permission = "organization.read"
	PermOrganizationCreate    Permission = "organization.create"
	PermOrganizationUpdate    Permission = "organization.update"
	PermOrganizationLifecycle Permission = "organization.lifecycle"

	// person
	PermPersonRead       Permission = "person.read"
	PermPersonCreate     Permission = "person.create"
	PermPersonUpdate     Permission = "person.update"
	PermPersonRankAssign Permission = "person.rank.assign"
	PermPersonLifecycle  Permission = "person.lifecycle"
	PermPersonPurge      Permission = "person.purge"
	PermPersonMerge      Permission = "person.merge" // resolve a provisional stub into a canonical person (D-OverlayFoundation, M29)

	// person pii:special sub-resources — each Art.9 read has its OWN code so no single grant
	// unlocks the aggregation of ethnicity + politics + party (D-DataScope, review R-14). These are
	// deliberately NOT in the base unit-reader set; they compose the sensitive-reader base role.
	PermPersonEthnicityRead        Permission = "person.ethnicity.read"
	PermPersonPoliticalLeaningRead Permission = "person.political_leaning.read"
	PermPersonPartyMembershipRead  Permission = "person.party_membership.read"
	PermPersonHealthRead           Permission = "person.health.read" // category-level health/vulnerability (M36)
	// Criminal / arrest / court records (D-LegalRecords, M38; GDPR Art. 10). Two codes: the base
	// need-to-know read (in sensitive-reader), and read-suppressed, which additionally reveals
	// sealed/expunged records — the strictest gate, deliberately in NO base role (grant it explicitly).
	PermPersonLegalRecordRead           Permission = "person.legal-record.read"
	PermPersonLegalRecordReadSuppressed Permission = "person.legal-record.read-suppressed"

	// person relationship graph (D-LinkPermissions) — the person↔person social/family links and the
	// person's home address. Each reified relationship gets its OWN read code, gating BOTH its dedicated
	// list endpoint AND the generic link-traversal arm (D-LinkTraversal), so the person page and the
	// graph can never disagree. Like the Art.9 set these are pii:sensitive and deliberately NOT in
	// unit-reader: knowing WHO someone is related to (and where they live) is a separate disclosure from
	// basic directory data. They compose the additive person-relationship-reader base role.
	PermPersonPartnershipRead  Permission = "person.partnership.read"
	PermPersonKinshipRead      Permission = "person.kinship.read"
	PermPersonGuardianshipRead Permission = "person.guardianship.read"
	PermPersonSponsorshipRead  Permission = "person.sponsorship.read"
	PermPersonNextOfKinRead    Permission = "person.next_of_kin.read"
	PermPersonAssociationRead  Permission = "person.association.read"
	PermPersonAddressRead      Permission = "person.address.read"

	// membership
	PermMembershipRead   Permission = "membership.read"
	PermMembershipCreate Permission = "membership.create"
	PermMembershipUpdate Permission = "membership.update"

	// position (unit-scoped — billets belong to a unit)
	PermPositionRead   Permission = "position.read"
	PermPositionCreate Permission = "position.create"
	PermPositionUpdate Permission = "position.update"

	// document (scoped through the holder; D-PersonReadScope / D-Documents)
	PermDocumentRead     Permission = "document.read"
	PermDocumentCreate   Permission = "document.create"
	PermDocumentUpdate   Permission = "document.update"
	PermDocumentDelete   Permission = "document.delete"
	PermDocumentTypeRead Permission = "document.type.read"

	// personal-code (national identifiers, pii:sensitive; scoped through the holder; D-PersonalCodes)
	PermPersonalCodeRead       Permission = "personal-code.read"
	PermPersonalCodeCreate     Permission = "personal-code.create"
	PermPersonalCodeUpdate     Permission = "personal-code.update"
	PermPersonalCodeDelete     Permission = "personal-code.delete"
	PermPersonalCodeSchemeRead Permission = "personal-code-scheme.read"

	// order (unit-scoped on the issuing unit; D-Orders)
	PermOrderRead     Permission = "order.read"
	PermOrderCreate   Permission = "order.create"
	PermOrderIssue    Permission = "order.issue"
	PermOrderRevoke   Permission = "order.revoke"
	PermOrderTypeRead Permission = "order.type.read"

	// authz
	PermRoleRead         Permission = "role.read"
	PermRoleCreate       Permission = "role.create"
	PermRoleUpdate       Permission = "role.update"
	PermRoleDelete       Permission = "role.delete"
	PermAssignmentRead   Permission = "assignment.read"
	PermAssignmentGrant  Permission = "assignment.grant"
	PermAssignmentRevoke Permission = "assignment.revoke"

	// audit
	PermAuditRead Permission = "audit.read"

	// rank
	PermRankSchemeRead Permission = "rank.scheme.read"

	// graph (D-Graphs)
	PermGraphRead Permission = "graph.read"

	// geo (D-Geo) — read the RID-keyed country registry so clients can resolve a country to its RID.
	PermCountryRead Permission = "country.read"

	// language (D-Languages, M18) — read the Glottolog languoid + writing-system registry.
	PermLanguageRead Permission = "language.read"

	// location (D-Location, M19) — the shared standalone place entity (PostGIS point + address). It
	// is instance-global (a location carries no unit scope of its own; access scoping is the owning
	// link's job), so reads/writes are satisfied anywhere via the PEP.
	PermLocationRead   Permission = "location.read"
	PermLocationCreate Permission = "location.create"
	PermLocationUpdate Permission = "location.update"

	// education (D-Education, M20) — external reference institutions + structure (units/buildings/
	// groups) + the person bindings (enrollments, dorm stays, positions/appointments). External
	// reference data, not tenant-unit scoped, so reads/writes are satisfied anywhere via the PEP.
	PermEducationRead             Permission = "education.read"
	PermEducationManage           Permission = "education.manage"
	PermEducationPositionManage   Permission = "education.position.manage"
	PermEducationEnrollmentManage Permission = "education.enrollment.manage"

	// company (D-Companies, M21) — legal-entity registry: companies + registrations + industries +
	// locations + the ownership/affiliation graph (foundings/shareholdings/beneficiaries/successions/
	// branches), plus positions/appointments. External reference data, not tenant-unit scoped, so
	// reads/writes are satisfied anywhere via the PEP.
	PermCompanyRead           Permission = "company.read"
	PermCompanyManage         Permission = "company.manage"
	PermCompanyPositionManage Permission = "company.position.manage"

	// vehicle (D-Vehicles, M26) — vehicle registry: brand/model/type catalogs + vehicles + the brand→
	// manufacturer link + the ownership/plate registration record. External reference data, not
	// tenant-unit scoped, so reads/writes are satisfied anywhere via the PEP. Catalog writes ride the
	// instance-plane `vehicle.catalog.manage` (below).
	PermVehicleRead   Permission = "vehicle.read"
	PermVehicleManage Permission = "vehicle.manage"
	// The vehicle↔owner registration link (D-LinkPermissions): vehicle.read lists vehicles, but WHO a
	// vehicle is registered to is a separate disclosure. Gates ListRegistrations + ListPersonVehicles and
	// the registered_to traversal arm; composes the additive vehicle-graph-reader base role. (The
	// brand→manufacturer link stays on vehicle.read — reference data, not ownership.)
	PermVehicleRegistrationRead Permission = "vehicle.registration.read"

	// finance (D-Finance, M44) — bank accounts + payment cards. Authoritative first-party directory data
	// (not tenant-unit scoped): reads/writes are satisfied anywhere via the PEP; person-held rows are
	// additionally holder-scoped (D-PersonReadScope). Catalog writes ride the instance-plane
	// `finance.catalog.manage` (below). Holding an account never grants authority (parallel to D-Rank).
	PermFinanceRead   Permission = "finance.read"
	PermFinanceManage Permission = "finance.manage"
	// The account↔holder ownership link (D-LinkPermissions): finance.read lists accounts/cards, but WHO
	// holds an account is a separate disclosure. Gates ListAccountHolders + ListPersonAccounts and the
	// held_by traversal arm; composes the additive finance-graph-reader base role.
	PermFinanceHolderRead Permission = "finance.holder.read"

	// religion (D-Religion, M22) — the multi-faith taxonomy (religion_taxa + closure) + the per-faith
	// catalogs are instance-global reference data (read anywhere; catalog writes on the instance plane
	// as `religion.catalog.manage`, below). The per-unit organization attributes (profile/
	// classifications/policies) ARE tenant-unit scoped: `religionorg.manage` is checked against the
	// religious-body unit over the canonical graph (authority cascades a governance subtree).
	PermReligionRead      Permission = "religion.read"
	PermReligionOrgManage Permission = "religionorg.manage"
	// clergy/affiliation (D-ClergyCredential M23 / D-ReligiousAffiliation M24) — person↔religion links.
	// A clergy credential is a public directory fact: `clergy.manage` is checked against the conferring
	// organization unit over the canonical graph (parallel to `religionorg.manage`). A lay affiliation is
	// `pii:special` person data: `affiliation.manage` is a person-data write satisfied anywhere via the
	// PEP (parallel to person updates). Neither is ever an authorization input (parallel to D-Rank).
	PermClergyManage      Permission = "clergy.manage"
	PermAffiliationManage Permission = "affiliation.manage"
	// discovery (D-Religion discovery surface, M25) — a site (worship-community unit ↔ a shared
	// location) and its service schedules are unit-scoped directory data: `site.manage` /
	// `schedule.manage` are checked against the organization unit over the canonical graph (parallel
	// to `religionorg.manage`, so authority cascades a governance subtree). Site/service-type catalog
	// writes ride the instance-plane `religion.catalog.manage`; discovery reads ride `religion.read`.
	PermSiteManage     Permission = "site.manage"
	PermScheduleManage Permission = "schedule.manage"

	// legal basis (D-OverlayFoundation, M29) — the structured GDPR lawful-basis catalog referenced by
	// every future pii:special overlay store. Instance-global reference data: read anywhere via the PEP;
	// catalog writes ride the instance-plane `legal-basis.manage` (below).
	PermLegalBasisRead Permission = "legal-basis.read"

	// color (D-Color) — the per-domain color catalog (eye/hair/vehicle), referenced by hard FK from
	// physical descriptions + vehicles. Instance-global reference data: read anywhere via the PEP (so
	// any reader can populate a color picker); catalog writes ride the instance-plane `color.manage`.
	PermColorRead Permission = "color.read"

	// external organizations (D-ExternalOrgs, M30) — the registry of external orgs a person is tied to
	// (parties, government bodies, foreign military, NGOs, registrants). Instance-global reference data,
	// not tenant-unit scoped: read anywhere via the PEP; catalog + org writes ride the instance-plane
	// `externalorg.manage` (below).
	PermExternalOrgRead Permission = "externalorg.read"

	// i18n
	PermLocaleRead        Permission = "locale.read"
	PermTranslationRead   Permission = "translation.read"
	PermLocaleManage      Permission = "locale.manage"
	PermTranslationManage Permission = "translation.manage"

	// instance-scope (only meaningful on the instance-admin plane; never in a base role)
	PermRankSchemeManage         Permission = "rank.scheme.manage"
	PermGraphManage              Permission = "graph.manage"
	PermClosureRebuild           Permission = "closure.rebuild"
	PermDocumentTypeManage       Permission = "document.type.manage"
	PermOrderTypeManage          Permission = "order.type.manage"
	PermDomainManage             Permission = "domain.manage"    // instance-admin manages the org-kind catalog (D-TenantOrganizations, M40)
	PermUnitKindManage           Permission = "unit-kind.manage" // instance-admin manages the domain-scoped unit-kind catalog (M40)
	PermPersonalCodeSchemeManage Permission = "personal-code-scheme.manage"
	PermCountryManage            Permission = "country.manage"
	PermLocationTypesManage      Permission = "location.types.manage"
	PermEducationCatalogManage   Permission = "education.catalog.manage"
	PermCompanyCatalogManage     Permission = "company.catalog.manage"
	PermVehicleCatalogManage     Permission = "vehicle.catalog.manage"
	PermReligionCatalogManage    Permission = "religion.catalog.manage"
	PermLegalBasisManage         Permission = "legal-basis.manage"     // instance-admin manages the GDPR lawful-basis catalog (D-OverlayFoundation, M29)
	PermColorManage              Permission = "color.manage"           // instance-admin manages the per-domain color catalog (D-Color)
	PermExternalOrgManage        Permission = "externalorg.manage"     // instance-admin manages the external-organizations registry (D-ExternalOrgs, M30)
	PermFinanceCatalogManage     Permission = "finance.catalog.manage" // instance-admin manages the account-type / card-network catalogs (D-Finance, M44)
	PermInstanceConfig           Permission = "instance.config"
	PermInstanceAdminManage      Permission = "instance.admin.manage"
	// import — the generic reference-data import endpoint (M16 / D-Hermenea). Held by a service
	// principal as a per-principal grant (M51 / D-ServiceIdentities) and grantable to a human instance
	// admin; the PDP only satisfies it on the instance plane.
	PermImportManage Permission = "import.manage"

	// service-principal — the machine-subject registry + its grants (M51 / D-ServiceIdentities).
	// Minting a machine identity is instance-admin-only: these gate the registry endpoints on
	// identity-federation AND the principal-grant endpoints on authorization.
	PermServicePrincipalRead   Permission = "service-principal.read"
	PermServicePrincipalManage Permission = "service-principal.manage"

	// account.security-log — the first-party login/IP security log (M37 / D-LoginSecurityLog).
	// Instance-scope: reading any account's login history is an admin act (the data is pii:contact).
	PermAccountSecurityLogRead Permission = "account.security-log.read"

	// connector plane (M53 / D-ConnectorPlane). The self-service codes are held by MACHINE subjects as
	// per-principal grants; `connector.read` is the operator fleet-view code. All instance-scope: a
	// connector is instance infrastructure with no unit or (in M53) organization dimension.
	PermConnectorRegister Permission = "connector.register" // a connector self-registers itself + its sources
	PermConnectorReport   Permission = "connector.report"   // a connector reports its sync runs
	PermConnectorRead     Permission = "connector.read"     // operators read the fleet (instance admin)
	// wiring API (M53 / D-ConnectorPlane, pull-wiring mode). Narrow READ surfaces a connector uses to
	// map its data before pushing. Each is its own code — what a connector may see is a grant, not a
	// default. Instance-scope: they read instance-wide reference data. Org-confined wiring is M55.
	PermWiringResolve     Permission = "wiring.resolve"      // resolve natural keys to RIDs
	PermWiringCatalogRead Permission = "wiring.catalog.read" // read reference catalogs (countries, languoids, …)
	PermWiringCursorRead  Permission = "wiring.cursor.read"  // read the connector's own registry row + cursors
)

// instanceScope is the set of permissions only meaningful on the instance-admin plane
// (D-InstanceAdmin / D-BaseRoles): they are unit-independent and are never granted via a unit
// assignment. The PDP only ever satisfies these from an active instance-admin grant.
var instanceScope = map[Permission]struct{}{
	PermRoleCreate:               {},
	PermRoleUpdate:               {},
	PermRoleDelete:               {},
	PermRankSchemeManage:         {},
	PermGraphManage:              {},
	PermClosureRebuild:           {},
	PermDocumentTypeManage:       {},
	PermOrderTypeManage:          {},
	PermDomainManage:             {},
	PermUnitKindManage:           {},
	PermPersonalCodeSchemeManage: {},
	PermCountryManage:            {},
	PermLocationTypesManage:      {},
	PermEducationCatalogManage:   {},
	PermCompanyCatalogManage:     {},
	PermVehicleCatalogManage:     {},
	PermReligionCatalogManage:    {},
	PermLegalBasisManage:         {},
	PermColorManage:              {},
	PermExternalOrgManage:        {},
	PermFinanceCatalogManage:     {},
	PermLocaleManage:             {},
	PermTranslationManage:        {},
	PermInstanceConfig:           {},
	PermInstanceAdminManage:      {},
	PermImportManage:             {},
	PermServicePrincipalRead:     {},
	PermServicePrincipalManage:   {},
	PermAccountSecurityLogRead:   {},
	PermConnectorRegister:        {},
	PermConnectorReport:          {},
	PermConnectorRead:            {},
	PermWiringResolve:            {},
	PermWiringCatalogRead:        {},
	PermWiringCursorRead:         {},
}

// catalog is the closed vocabulary — the union of every permission constant above. It is the
// validation set for authz_role_permissions writes and the membership of `assignment`-level reads.
var catalog = func() map[Permission]struct{} {
	all := []Permission{
		PermUnitRead, PermUnitCreate, PermUnitUpdate, PermUnitRecode, PermUnitLifecycle,
		PermUnitEdgesManage, PermUnitEdgesCommandManage, PermUnitEdgesOperationalManage,
		PermDomainRead, PermUnitKindRead, PermOrganizationRead, PermOrganizationCreate, PermOrganizationUpdate, PermOrganizationLifecycle,
		PermDomainManage, PermUnitKindManage,
		PermPersonRead, PermPersonCreate, PermPersonUpdate, PermPersonRankAssign, PermPersonLifecycle, PermPersonPurge, PermPersonMerge,
		PermPersonEthnicityRead, PermPersonPoliticalLeaningRead, PermPersonPartyMembershipRead, PermPersonHealthRead,
		PermPersonLegalRecordRead, PermPersonLegalRecordReadSuppressed,
		PermPersonPartnershipRead, PermPersonKinshipRead, PermPersonGuardianshipRead, PermPersonSponsorshipRead,
		PermPersonNextOfKinRead, PermPersonAssociationRead, PermPersonAddressRead,
		PermMembershipRead, PermMembershipCreate, PermMembershipUpdate,
		PermPositionRead, PermPositionCreate, PermPositionUpdate,
		PermDocumentRead, PermDocumentCreate, PermDocumentUpdate, PermDocumentDelete, PermDocumentTypeRead,
		PermPersonalCodeRead, PermPersonalCodeCreate, PermPersonalCodeUpdate, PermPersonalCodeDelete, PermPersonalCodeSchemeRead,
		PermOrderRead, PermOrderCreate, PermOrderIssue, PermOrderRevoke, PermOrderTypeRead,
		PermRoleRead, PermRoleCreate, PermRoleUpdate, PermRoleDelete, PermAssignmentRead, PermAssignmentGrant, PermAssignmentRevoke,
		PermAuditRead,
		PermRankSchemeRead,
		PermGraphRead,
		PermCountryRead,
		PermLanguageRead,
		PermLocationRead, PermLocationCreate, PermLocationUpdate,
		PermEducationRead, PermEducationManage, PermEducationPositionManage, PermEducationEnrollmentManage,
		PermCompanyRead, PermCompanyManage, PermCompanyPositionManage,
		PermVehicleRead, PermVehicleManage, PermVehicleRegistrationRead,
		PermFinanceRead, PermFinanceManage, PermFinanceHolderRead,
		PermReligionRead, PermReligionOrgManage, PermClergyManage, PermAffiliationManage, PermSiteManage, PermScheduleManage,
		PermLegalBasisRead, PermLegalBasisManage,
		PermColorRead, PermColorManage,
		PermExternalOrgRead, PermExternalOrgManage,
		PermLocaleRead, PermTranslationRead, PermLocaleManage, PermTranslationManage,
		PermRankSchemeManage, PermGraphManage, PermClosureRebuild, PermDocumentTypeManage, PermOrderTypeManage,
		PermPersonalCodeSchemeManage, PermCountryManage, PermLocationTypesManage, PermEducationCatalogManage, PermCompanyCatalogManage, PermVehicleCatalogManage, PermFinanceCatalogManage, PermReligionCatalogManage, PermInstanceConfig, PermInstanceAdminManage,
		PermImportManage,
		PermServicePrincipalRead, PermServicePrincipalManage,
		PermAccountSecurityLogRead,
		PermConnectorRegister, PermConnectorReport, PermConnectorRead,
		PermWiringResolve, PermWiringCatalogRead, PermWiringCursorRead,
	}
	m := make(map[Permission]struct{}, len(all))
	for _, p := range all {
		m[p] = struct{}{}
	}
	return m
}()

// IsKnownPermission reports whether code is in the closed catalog. A write to
// authz_role_permissions with an unknown code is rejected (the authorization surface is in a diff).
func IsKnownPermission(code string) bool {
	_, ok := catalog[Permission(code)]
	return ok
}

// IsInstanceScope reports whether code is an instance-plane-only permission. The PDP satisfies these
// solely from an active instance-admin grant, never from a unit assignment.
func IsInstanceScope(code string) bool {
	_, ok := instanceScope[Permission(code)]
	return ok
}

// Catalog returns the full closed permission vocabulary (sorted is not guaranteed; callers that need
// order should sort). Used by tooling/introspection and the seed validation.
func Catalog() []Permission {
	out := make([]Permission, 0, len(catalog))
	for p := range catalog {
		out = append(out, p)
	}
	return out
}

// BaseRole is a seeded, code-defined role (D-BaseRoles). Base roles are unit-scoped (assignable with
// `unit` or `subtree` scope) and immutable by instance admins (is_base). The four graduate like the
// Kubernetes view/edit/admin defaults.
type BaseRole struct {
	Code        string
	Name        string
	Description string
	Permissions []Permission
}

// Base role codes (seeded; immutable by convention).
const (
	BaseRoleUnitReader               = "unit-reader"
	BaseRoleUnitManager              = "unit-manager"
	BaseRoleUnitAdmin                = "unit-admin"
	BaseRoleAuditor                  = "auditor"
	BaseRoleSensitiveReader          = "sensitive-reader"
	BaseRolePersonRelationshipReader = "person-relationship-reader"
	BaseRoleFinanceGraphReader       = "finance-graph-reader"
	BaseRoleVehicleGraphReader       = "vehicle-graph-reader"
)

// readerPerms / managerOnlyPerms / adminOnlyPerms compose the graduated base-role sets
// (D-BaseRoles + the document/personal-code/order amendments in authorization.md). manager = reader
// + managerOnly; admin = manager + adminOnly. None contains an instance-scope permission.
var readerPerms = []Permission{
	PermUnitRead, PermPersonRead, PermMembershipRead, PermPositionRead,
	PermDocumentRead, PermPersonalCodeRead, PermOrderRead,
	PermRoleRead, PermAssignmentRead,
	PermRankSchemeRead, PermGraphRead, PermCountryRead, PermLanguageRead, PermLocationRead,
	PermDomainRead, PermUnitKindRead, PermOrganizationRead,
	PermDocumentTypeRead, PermPersonalCodeSchemeRead, PermOrderTypeRead,
	PermReligionRead, PermLegalBasisRead, PermColorRead, PermExternalOrgRead,
	PermLocaleRead, PermTranslationRead,
}

var managerOnlyPerms = []Permission{
	PermUnitCreate, PermUnitUpdate,
	PermPersonCreate, PermPersonUpdate, PermPersonRankAssign,
	PermMembershipCreate, PermMembershipUpdate,
	PermPositionCreate, PermPositionUpdate,
	PermDocumentCreate, PermDocumentUpdate,
	PermPersonalCodeCreate, PermPersonalCodeUpdate,
	PermOrderCreate,
	PermLocationCreate, PermLocationUpdate,
	PermOrganizationCreate, PermOrganizationUpdate,
	PermReligionOrgManage, PermSiteManage, PermScheduleManage,
}

// sensitiveReaderPerms is a standalone, additive base role (like auditor): the pii:special person
// reads, deliberately NOT folded into reader/manager/admin so that no single graduated role unlocks
// the ethnicity + politics + party aggregation — reading Art.9 person data requires this explicit
// grant, on top of unit-reader for the surrounding directory context (D-DataScope, review R-14).
var sensitiveReaderPerms = []Permission{
	PermPersonEthnicityRead, PermPersonPoliticalLeaningRead, PermPersonPartyMembershipRead, PermPersonHealthRead,
	// The base legal-record read (D-LegalRecords, M38). read-suppressed is deliberately NOT here —
	// sealed/expunged records require an explicit, separately granted capability (the strictest gate).
	PermPersonLegalRecordRead,
}

// personRelationshipReaderPerms is the additive base role for the person relationship graph
// (D-LinkPermissions): who a person is partnered with / related to / guardian of / vouched for / lists
// as next of kin / associates with, and where they live. Held together because they are one coherent
// disclosure (the personal graph), but each is its own code so a deployment can compose a narrower role.
// Deliberately NOT folded into reader/manager/admin: the relationship graph is a separate disclosure
// from directory data, exactly like the Art.9 set (D-DataScope). Pair with unit-reader for context.
var personRelationshipReaderPerms = []Permission{
	PermPersonPartnershipRead, PermPersonKinshipRead, PermPersonGuardianshipRead,
	PermPersonSponsorshipRead, PermPersonNextOfKinRead, PermPersonAssociationRead,
	PermPersonAddressRead,
}

// financeGraphReaderPerms / vehicleGraphReaderPerms are the per-module ownership-link roles
// (D-LinkPermissions): the module read lists the assets, these disclose WHO holds/owns them. Additive
// and per-module so "can see bank accounts exist" and "can see whose they are" are separate grants.
var financeGraphReaderPerms = []Permission{PermFinanceHolderRead}

var vehicleGraphReaderPerms = []Permission{PermVehicleRegistrationRead}

var adminOnlyPerms = []Permission{
	PermUnitEdgesManage, // broad form — covers all graphs incl. custom (D-EdgePerms)
	PermUnitLifecycle,
	PermOrganizationLifecycle, // organization suspend/archive/restore (D-TenantOrganizations, M40)
	PermUnitRecode,            // changing the external handle is a privileged action (D-UnitCodeLifecycle, M28)
	PermPersonLifecycle, PermPersonPurge, PermPersonMerge,
	PermDocumentDelete, PermPersonalCodeDelete,
	PermOrderIssue, PermOrderRevoke,
	PermAssignmentGrant, PermAssignmentRevoke,
}

// BaseRoles returns the four seeded base roles with their composed permission sets. The order is
// reader → manager → admin → auditor (graduated). Seeded idempotently at boot (D-RIDSeeding).
func BaseRoles() []BaseRole {
	manager := concat(readerPerms, managerOnlyPerms)
	admin := concat(manager, adminOnlyPerms)
	return []BaseRole{
		{Code: BaseRoleUnitReader, Name: "Unit Reader", Description: "Read-only access within scope.", Permissions: readerPerms},
		{Code: BaseRoleUnitManager, Name: "Unit Manager", Description: "Create/update people, memberships, positions, units, and orders within scope.", Permissions: manager},
		{Code: BaseRoleUnitAdmin, Name: "Unit Admin", Description: "Full unit administration within scope: edges, lifecycle, purge, order issue/revoke, and granting assignments.", Permissions: admin},
		{Code: BaseRoleAuditor, Name: "Auditor", Description: "Read the audit log only (separation of duties; pair with unit-reader to resolve referenced entities).", Permissions: []Permission{PermAuditRead}},
		{Code: BaseRoleSensitiveReader, Name: "Sensitive Reader", Description: "Read a person's pii:special Art.9/Art.10 data (ethnicity, inferred political leaning, party membership, category-level health & vulnerability, and criminal/arrest/court records — excluding sealed/expunged records, which need person.legal-record.read-suppressed). Additive and explicit — pair with unit-reader; deliberately not implied by unit-admin (D-DataScope, R-14).", Permissions: sensitiveReaderPerms},
		{Code: BaseRolePersonRelationshipReader, Name: "Person Relationship Reader", Description: "Read a person's relationship graph (partnerships, kinships, guardianships, sponsorships, next of kin, associations) and home addresses — on the person page and in the object graph alike. Additive and explicit — pair with unit-reader; deliberately not implied by unit-admin (D-LinkPermissions).", Permissions: personRelationshipReaderPerms},
		{Code: BaseRoleFinanceGraphReader, Name: "Finance Graph Reader", Description: "Read who holds a bank account (the account↔holder ownership link), on the account/person pages and in the object graph alike. Additive — pair with finance.read, which lists the accounts themselves (D-LinkPermissions).", Permissions: financeGraphReaderPerms},
		{Code: BaseRoleVehicleGraphReader, Name: "Vehicle Graph Reader", Description: "Read who a vehicle is registered to (the vehicle↔owner link), on the vehicle/person pages and in the object graph alike. Additive — pair with vehicle.read, which lists the vehicles themselves (D-LinkPermissions).", Permissions: vehicleGraphReaderPerms},
	}
}

func concat(a, b []Permission) []Permission {
	out := make([]Permission, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

// SensitiveReadPermissions returns the pii:special person-read permission set that composes the
// sensitive-reader base role (D-DataScope, R-14). Holding all of them (or being an instance admin)
// is the "sensitive-reader capability" the audit object-history endpoint requires before it reveals
// before/after change payloads (D-Temporal, R-31) — the same bar as reading that Art.9 data directly,
// so the audit projection can never leak special-category PII to a subject who couldn't read it.
func SensitiveReadPermissions() []Permission {
	out := make([]Permission, len(sensitiveReaderPerms))
	copy(out, sensitiveReaderPerms)
	return out
}
