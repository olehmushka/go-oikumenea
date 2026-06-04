package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB constraints
// (active-uniqueness, graph/scope CHECK, RESTRICT FKs) enforce the same shapes as a backstop.
var (
	ErrRoleNotFound      = errors.New("role not found")
	ErrRoleCodeConflict  = errors.New("role code already exists")
	ErrRoleInvalid       = errors.New("invalid role request")
	ErrRoleIsBase        = errors.New("base roles are immutable")
	ErrRoleInUse         = errors.New("role is assigned and cannot be deleted")
	ErrUnknownPermission = errors.New("unknown permission code")

	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrAssignmentConflict = errors.New("an identical active assignment already exists")
	ErrAssignmentInvalid  = errors.New("invalid assignment request")
	// ErrNonAuthorityBearingGraph: a subtree grant named a directory-only graph (D-DirectoryGraphs).
	ErrNonAuthorityBearingGraph = errors.New("graph is directory-only; subtree grants are not permitted")
	// ErrSelfEscalation: the grantor lacks the authority to grant this (no self-escalation).
	ErrSelfEscalation = errors.New("grantor lacks authority to grant this assignment")

	ErrInstanceAdminNotFound = errors.New("instance-admin grant not found")
	ErrInstanceAdminConflict = errors.New("the person is already an active instance admin")

	// FK-validation sentinels: unknown references caught by the DB foreign keys.
	ErrUnknownSubject = errors.New("subject person does not exist")
	ErrUnknownRole    = errors.New("role does not exist")
	ErrUnknownUnit    = errors.New("target unit does not exist")
	ErrUnknownGraph   = errors.New("graph does not exist")

	// ErrPermissionDenied is the PDP's negative decision surfaced over the HTTP boundary.
	ErrPermissionDenied = errors.New("permission denied")
)

// Scope is a role assignment's reach (D-Inherit). `unit` applies at target only; `subtree` cascades
// to target + all descendants in the assignment's graph.
type Scope string

const (
	ScopeUnit    Scope = "unit"
	ScopeSubtree Scope = "subtree"
)

// Role is a named set of permission codes (Object Role). Name/Description are default-locale
// fallbacks; the transport assembles their locale->text maps from the i18n store. Permissions is the
// role's code membership (validated against the catalog).
type Role struct {
	ID          string
	Code        string
	Name        string
	Description string
	IsBase      bool
	Permissions []Permission
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate enforces the create-time invariants for a custom role: a valid code, a non-empty name, and
// a permission set drawn entirely from the catalog and containing NO instance-scope permission
// (instance-scope authority is held only on the instance-admin plane, never via a role assignment).
func (r Role) Validate() error {
	if !validCode(r.Code) {
		return wrapInvalid(ErrRoleInvalid, "code must be non-empty, <=128 chars, and contain no whitespace")
	}
	if strings.TrimSpace(r.Name) == "" {
		return wrapInvalid(ErrRoleInvalid, "name is required")
	}
	return ValidatePermissionSet(r.Permissions)
}

// ValidatePermissionSet rejects unknown and instance-scope permission codes. A role's permissions are
// always unit-scoped; instance-scope permissions belong to the instance-admin plane (D-InstanceAdmin).
func ValidatePermissionSet(perms []Permission) error {
	for _, p := range perms {
		if !IsKnownPermission(string(p)) {
			return errors.Join(ErrUnknownPermission, errors.New(string(p)))
		}
		if IsInstanceScope(string(p)) {
			return errors.Join(ErrRoleInvalid, errors.New("instance-scope permission not allowed in a role: "+string(p)))
		}
	}
	return nil
}

// RolePatch is a partial update of a custom role (nil = unchanged). Code and is_base are immutable.
type RolePatch struct {
	Name        *string
	Description *string
	Permissions *[]Permission // full replacement of the permission set when present
}

// Assignment is the reified link__has_role — the unit of granted authority (D-Inherit / D-Graphs).
// GraphID is "" iff Scope == unit. RevokedAt/ExpiresAt nil = not revoked / no expiry.
type Assignment struct {
	ID              string
	SubjectPersonID string
	RoleID          string
	TargetUnitID    string
	Scope           Scope
	GraphID         string // "" for unit scope; the cascade graph (RID) for subtree
	GrantedBy       string // "" for the bootstrap grant
	GrantedAt       time.Time
	RevokedAt       *time.Time
	RevokedBy       string
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Active reports whether the assignment is currently in force: not revoked and not expired
// (D-TimeBoundGrants — silent, decision-time expiry).
func (a Assignment) Active(now time.Time) bool {
	if a.RevokedAt != nil {
		return false
	}
	if a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
		return false
	}
	return true
}

// InstanceAdmin is the reified link__instance_admin — a person on the instance-wide authority plane.
type InstanceAdmin struct {
	ID        string
	PersonID  string
	GrantedBy string // "" for the bootstrap grant
	GrantedAt time.Time
	RevokedAt *time.Time
	RevokedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GrantInput is a validated request to grant an assignment. GraphCode is the named graph for a
// subtree grant (empty defaults to the command graph); it is ignored for unit scope.
type GrantInput struct {
	SubjectPersonID string
	RoleID          string
	TargetUnitID    string
	Scope           Scope
	GraphCode       string // subtree only; "" => default (command)
	GrantedBy       string // resolved acting person; "" for bootstrap
	ExpiresAt       *time.Time
}

// Validate enforces the assignment-shape invariants the PDP relies on. The graph/scope pairing is
// finalized in the application once the graph is resolved (subtree => non-empty graph id).
func (g GrantInput) Validate() error {
	if strings.TrimSpace(g.SubjectPersonID) == "" {
		return wrapInvalid(ErrAssignmentInvalid, "subjectPersonId is required")
	}
	if strings.TrimSpace(g.RoleID) == "" {
		return wrapInvalid(ErrAssignmentInvalid, "roleId is required")
	}
	if strings.TrimSpace(g.TargetUnitID) == "" {
		return wrapInvalid(ErrAssignmentInvalid, "targetUnitId is required")
	}
	switch g.Scope {
	case ScopeUnit, ScopeSubtree:
	default:
		return wrapInvalid(ErrAssignmentInvalid, "scope must be one of unit|subtree")
	}
	return nil
}

// ClosurePort is the tenant-graph closure surface the PDP depends on (cross-module query —
// overview.md). The tenant module implements it. All lookups are keyed by graph RID, matching the
// graph_id carried on the assignment.
type ClosurePort interface {
	// IsAncestorOrSelf reports whether ancestorUnitID reaches descendantUnitID in the graph's
	// closure. The PDP additionally treats target == unit as self-authorized WITHOUT this call
	// (the reflexive closure row exists only for units that appear in the graph's edges).
	IsAncestorOrSelf(ctx context.Context, graphID, ancestorUnitID, descendantUnitID string) (bool, error)
	// IsAuthorityBearing reports whether the graph cascades authority (D-DirectoryGraphs). A subtree
	// grant on a directory-only graph confers nothing in the PDP.
	IsAuthorityBearing(ctx context.Context, graphID string) (bool, error)
	// DescendantUnitIDs returns the strict descendants (excludes self) of unitID in the graph's
	// closure — used to expand a subtree grant into the effective read/write unit-set
	// (D-RLSDefenseInDepth).
	DescendantUnitIDs(ctx context.Context, graphID, unitID string) ([]string, error)
}

// Repository is the persistence port the application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). Reads exclude soft-deleted / revoked rows unless a
// method documents otherwise.
type Repository interface {
	// roles
	InsertRole(ctx context.Context, r Role) (Role, error)
	GetRole(ctx context.Context, id string) (Role, error)
	GetRoleByCode(ctx context.Context, code string) (Role, error)
	ListRoles(ctx context.Context, after string, limit int) ([]Role, error)
	UpdateRole(ctx context.Context, id string, patch RolePatch) (Role, error)
	SoftDeleteRole(ctx context.Context, id string) error
	RoleHasActiveAssignments(ctx context.Context, roleID string) (bool, error)
	ReplaceRolePermissions(ctx context.Context, roleID string, perms []Permission) error

	// assignments
	InsertAssignment(ctx context.Context, g GrantInput, graphID string) (Assignment, error)
	GetAssignment(ctx context.Context, id string) (Assignment, error)
	RevokeAssignment(ctx context.Context, id, revokedBy string) (Assignment, error)
	ListAssignmentsBySubject(ctx context.Context, subjectPersonID, after string, limit int) ([]Assignment, error)
	ListAssignmentsByUnit(ctx context.Context, targetUnitID, after string, limit int) ([]Assignment, error)
	// ActiveGrantsForSubject returns the subject's active (not revoked) assignments joined with each
	// role's permission codes, grouped into ActiveGrants. Decision-time expiry is applied by the PDP.
	ActiveGrantsForSubject(ctx context.Context, subjectPersonID string) ([]ActiveGrant, error)

	// instance admins
	InsertInstanceAdmin(ctx context.Context, personID, grantedBy string) (InstanceAdmin, error)
	GetInstanceAdmin(ctx context.Context, id string) (InstanceAdmin, error)
	RevokeInstanceAdmin(ctx context.Context, id, revokedBy string) (InstanceAdmin, error)
	IsActiveInstanceAdmin(ctx context.Context, personID string) (bool, error)
	// HasActiveInstanceAdmin reports whether ANY active instance admin exists (bootstrap idempotency
	// gate — D-Bootstrap).
	HasActiveInstanceAdmin(ctx context.Context) (bool, error)
}

func wrapInvalid(base error, msg string) error { return errors.Join(base, errors.New(msg)) }

// validCode is the shared code shape guard: non-empty, <=128 chars, no whitespace (D-Code).
func validCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	return !strings.ContainsFunc(code, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}
