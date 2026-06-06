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
	PermUnitLifecycle Permission = "unit.lifecycle"

	// unit edges — per graph (D-EdgePerms) + a broad fallback covering all graphs incl. custom.
	PermUnitEdgesManage            Permission = "unit.edges.manage"
	PermUnitEdgesCommandManage     Permission = "unit.edges.command.manage"
	PermUnitEdgesOperationalManage Permission = "unit.edges.operational.manage"

	// person
	PermPersonRead       Permission = "person.read"
	PermPersonCreate     Permission = "person.create"
	PermPersonUpdate     Permission = "person.update"
	PermPersonRankAssign Permission = "person.rank.assign"
	PermPersonLifecycle  Permission = "person.lifecycle"
	PermPersonPurge      Permission = "person.purge"

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
	PermPersonalCodeSchemeManage Permission = "personal-code-scheme.manage"
	PermCountryManage            Permission = "country.manage"
	PermInstanceConfig           Permission = "instance.config"
	PermInstanceAdminManage      Permission = "instance.admin.manage"
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
	PermPersonalCodeSchemeManage: {},
	PermCountryManage:            {},
	PermLocaleManage:             {},
	PermTranslationManage:        {},
	PermInstanceConfig:           {},
	PermInstanceAdminManage:      {},
}

// catalog is the closed vocabulary — the union of every permission constant above. It is the
// validation set for authz_role_permissions writes and the membership of `assignment`-level reads.
var catalog = func() map[Permission]struct{} {
	all := []Permission{
		PermUnitRead, PermUnitCreate, PermUnitUpdate, PermUnitLifecycle,
		PermUnitEdgesManage, PermUnitEdgesCommandManage, PermUnitEdgesOperationalManage,
		PermPersonRead, PermPersonCreate, PermPersonUpdate, PermPersonRankAssign, PermPersonLifecycle, PermPersonPurge,
		PermMembershipRead, PermMembershipCreate, PermMembershipUpdate,
		PermPositionRead, PermPositionCreate, PermPositionUpdate,
		PermDocumentRead, PermDocumentCreate, PermDocumentUpdate, PermDocumentDelete, PermDocumentTypeRead,
		PermPersonalCodeRead, PermPersonalCodeCreate, PermPersonalCodeUpdate, PermPersonalCodeDelete, PermPersonalCodeSchemeRead,
		PermOrderRead, PermOrderCreate, PermOrderIssue, PermOrderRevoke, PermOrderTypeRead,
		PermRoleRead, PermRoleCreate, PermRoleUpdate, PermRoleDelete, PermAssignmentRead, PermAssignmentGrant, PermAssignmentRevoke,
		PermAuditRead,
		PermRankSchemeRead,
		PermGraphRead,
		PermLocaleRead, PermTranslationRead, PermLocaleManage, PermTranslationManage,
		PermRankSchemeManage, PermGraphManage, PermClosureRebuild, PermDocumentTypeManage, PermOrderTypeManage,
		PermPersonalCodeSchemeManage, PermCountryManage, PermInstanceConfig, PermInstanceAdminManage,
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
	BaseRoleUnitReader  = "unit-reader"
	BaseRoleUnitManager = "unit-manager"
	BaseRoleUnitAdmin   = "unit-admin"
	BaseRoleAuditor     = "auditor"
)

// readerPerms / managerOnlyPerms / adminOnlyPerms compose the graduated base-role sets
// (D-BaseRoles + the document/personal-code/order amendments in authorization.md). manager = reader
// + managerOnly; admin = manager + adminOnly. None contains an instance-scope permission.
var readerPerms = []Permission{
	PermUnitRead, PermPersonRead, PermMembershipRead, PermPositionRead,
	PermDocumentRead, PermPersonalCodeRead, PermOrderRead,
	PermRoleRead, PermAssignmentRead,
	PermRankSchemeRead, PermGraphRead,
	PermDocumentTypeRead, PermPersonalCodeSchemeRead, PermOrderTypeRead,
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
}

var adminOnlyPerms = []Permission{
	PermUnitEdgesManage, // broad form — covers all graphs incl. custom (D-EdgePerms)
	PermUnitLifecycle,
	PermPersonLifecycle, PermPersonPurge,
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
	}
}

func concat(a, b []Permission) []Permission {
	out := make([]Permission, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}
