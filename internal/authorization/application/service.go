// Package application holds the authorization module's application service — the orchestrator the
// transport layer and (in-process) every other module's PEP call to decide and to mutate roles,
// assignments, and instance-admin grants, recording an audit row in the same transaction as each
// write (D-Audit). It depends on the domain port + PDP engine, the platform DB surface, the audit
// service, and a graph-resolution port (tenant), but never imports the adapters package directly.
//
// Authority comes ONLY from assignments here; the PDP reads no rank/position. The decision path is
// the product's centerpiece: Decide answers authorize(person, action, unit) by unioning the
// subject's active grants over the tenant closure (per-assignment scope × graph), plus the
// instance-admin plane. Since M47 (review-2026-07 R-01/R-02) authority state is snapshotted per
// request + cached per process (authority.go / grantcache.go), and reach is never materialized
// app-side — the shadow gate and the RLS backstop consume SQL reach predicates instead.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// pageSize is this module's page-size policy (M56 / pkg/listing): the shared clamp bound to the
// module's own Default/Max, replacing the per-module resolvePageSize copy.
var pageSizePolicy = listing.PageSize{Default: DefaultPageSize, Max: MaxPageSize}

// auditSubsystem labels the interim system actor for authz admin writes. Until identity-federation
// (M8) resolves the acting person from the validated token, writes whose grantor is not supplied are
// recorded as a `system` action under this subsystem (the no-unaudited-mutation rule still holds).
const auditSubsystem = "authz-admin"

// Audit target types (the audited entity kinds).
const (
	targetRole          = "role"
	targetAssignment    = "assignment"
	targetInstanceAdmin = "instance_admin"
)

// RepositoryFactory binds a domain.Repository to a command surface (pool for reads, a tx for an
// audited write). Injected by module.go so the application never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// GraphPort resolves a named graph for the grant path (cross-module query — tenant), scoped to the
// TARGET UNIT's organization (graphs are per-org — D-TenantOrganizations, M40), falling back to an
// instance-global graph. An empty code means the org's default (command). Returns the graph RID and
// whether it cascades authority.
type GraphPort interface {
	ResolveGraph(ctx context.Context, targetUnitID, code string) (graphID string, authorityBearing bool, err error)
}

// Service is the authorization application service. It owns its writes (holds the pool to open
// transactions); reads run on the pool directly. The PDP engine carries the tenant closure port.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	pdp     domain.PDP
	graphs  GraphPort
	grants  *grantCache // epoch-validated per-process authority cache (D-AuthzGrantCache)
	// The machine-subject authority plane (M51 / D-ServiceIdentities). newPrincipalRepo is injected
	// with the other factories; principals is the cross-module port into identity-federation's
	// registry, LATE-BOUND by main.go because that module registers after this one.
	newPrincipalRepo PrincipalRepositoryFactory
	principals       domain.PrincipalDirectory
	// disabledPrefixes are the permission-code prefixes of verticals switched off via modules.*.enabled
	// (D-DataPacks, M54). A role or principal grant naming a code under one of these is rejected as
	// effectively unknown — a disabled module's codes are not grantable, though they stay structurally
	// valid in the closed catalog (so the module re-enables by a config flip, no data change).
	disabledPrefixes []string
}

// SetDisabledModulePrefixes records which disabled modules' code prefixes are non-grantable
// (D-DataPacks, M54). Composition-time, from install config; empty (the default) grants everything the
// static catalog allows.
func (s *Service) SetDisabledModulePrefixes(prefixes []string) { s.disabledPrefixes = prefixes }

// rejectDisabledCode returns ErrUnknownPermission when a code belongs to a disabled module (M54), so a
// grant of it maps to the same "unknown permission" error an unregistered code does.
func (s *Service) rejectDisabledCode(code domain.Permission) error {
	for _, px := range s.disabledPrefixes {
		if strings.HasPrefix(string(code), px) {
			return errors.Join(domain.ErrUnknownPermission, errors.New(string(code)))
		}
	}
	return nil
}

// NewService wires the service with the pool, the repository factory, the audit service, the PDP
// engine (built over the tenant closure port), and the graph-resolution port.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, pdp domain.PDP, graphs GraphPort, newPrincipalRepo PrincipalRepositoryFactory) *Service {
	return &Service{
		pool: pool, newRepo: newRepo, audit: audit, pdp: pdp, graphs: graphs, grants: newGrantCache(),
		newPrincipalRepo: newPrincipalRepo,
	}
}

// BindPrincipalDirectory late-binds identity-federation's registry (the established seam pattern —
// cf. SetLocationLookup / SetWatchlistLookup). main.go calls it after identityfederation.Register
// and asserts it in the MustBeBound sweep, so a missing binding fails the boot rather than a
// request.
func (s *Service) BindPrincipalDirectory(d domain.PrincipalDirectory) { s.principals = d }

// PrincipalDirectoryBound reports whether the seam is wired (boot assertion).
func (s *Service) PrincipalDirectoryBound() bool { return s.principals != nil }

// RolePage / AssignmentPage are keyset-paginated slices plus the opaque next-page token.
type RolePage struct {
	Roles         []domain.Role
	NextPageToken string
}
type AssignmentPage struct {
	Assignments   []domain.Assignment
	NextPageToken string
}

// ============================ the PDP (decisions + reach) ============================

// Decide answers authorize(subject, action, unit). Authority state (instance-admin flag + active
// grants) comes from the request snapshot when one is attached (authority.go, R-01.1), else from a
// fresh fetch; either way the pure engine runs on a consistent snapshot. A revoke or role edit
// takes effect on the next authenticated request (see D-AuthzGrantCache for the exact bound).
func (s *Service) Decide(ctx context.Context, subjectPersonID, action, unitID string, explain bool) (domain.Decision, error) {
	isAdmin, grants, err := s.authorityFor(ctx, subjectPersonID)
	if err != nil {
		return domain.Decision{}, err
	}
	return s.pdp.Decide(ctx, domain.DecisionInput{
		Grants: grants, IsInstanceAdmin: isAdmin, Action: action, UnitID: unitID, Explain: explain,
	})
}

// BatchQuery is one question in a batch decision.
type BatchQuery struct {
	Action string
	UnitID string
}

// DecideBatch answers several questions for one subject, resolving the subject's authority state once.
func (s *Service) DecideBatch(ctx context.Context, subjectPersonID string, queries []BatchQuery, explain bool) ([]domain.Decision, error) {
	isAdmin, grants, err := s.authorityFor(ctx, subjectPersonID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Decision, 0, len(queries))
	for _, q := range queries {
		d, err := s.pdp.Decide(ctx, domain.DecisionInput{
			Grants: grants, IsInstanceAdmin: isAdmin, Action: q.Action, UnitID: q.UnitID, Explain: explain,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

// Enforce is the PEP entry every module's transport calls before a guarded op: it returns
// domain.ErrPermissionDenied when the decision is DENY, else nil. unitID is "" for instance-scope
// actions (the engine ignores it).
func (s *Service) Enforce(ctx context.Context, subjectPersonID, action, unitID string) error {
	d, err := s.Decide(ctx, subjectPersonID, action, unitID, false)
	if err != nil {
		return err
	}
	if !d.Allow {
		return domain.ErrPermissionDenied
	}
	return nil
}

// HoldsPermissionAnywhere reports whether the subject can satisfy action at ANY unit (or holds it on
// the instance plane). It is the gate for instance-global reads whose resource has no single unit
// (e.g. the role list): a unit-scoped read permission is required, but the resource is not
// unit-keyed, so "holds it somewhere" is the right question. Instance-scope actions are satisfied
// only by an instance admin.
func (s *Service) HoldsPermissionAnywhere(ctx context.Context, subjectPersonID, action string) (bool, error) {
	isAdmin, grants, err := s.authorityFor(ctx, subjectPersonID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	if domain.IsInstanceScope(action) {
		return false, nil // instance-scope perms live only on the instance plane
	}
	for _, g := range grants {
		if g.Has(domain.Permission(action)) {
			return true, nil
		}
	}
	return false, nil
}

// EffectivePermissions returns the flat, deduped, sorted union of the subject's active-grant
// permission codes plus whether they are an instance admin. It is the self-serve introspection
// primitive behind GET /me/capabilities (D-SelfCapabilities): a caller learning their OWN effective
// codes, never a (subject, action, unit) question. For an instance admin the union is not
// enumerated — the flag says "holds everything" and the caller (UI) treats it as show-all.
func (s *Service) EffectivePermissions(ctx context.Context, subjectPersonID string) ([]string, bool, error) {
	isAdmin, grants, err := s.authorityFor(ctx, subjectPersonID)
	if err != nil {
		return nil, false, err
	}
	if isAdmin {
		return nil, true, nil
	}
	set := map[domain.Permission]struct{}{}
	for _, g := range grants {
		for p := range g.Perms {
			set[p] = struct{}{}
		}
	}
	perms := make([]string, 0, len(set))
	for p := range set {
		perms = append(perms, string(p))
	}
	sort.Strings(perms)
	return perms, false, nil
}

// NOTE (review-2026-07 R-02): EffectiveReach and RLSStateFor are GONE. Nothing on the request path
// materializes the subject's reach app-side anymore: person/document read scope is a SQL semi-join
// (membership's VisiblePersonIDsForSubject / SubjectCanReadPerson), the shadow gate is a batch SQL
// probe (ReadableUnitsForSubjectAmong below), and the RLS backstop computes reach live in the
// policies (oikumenea.authz_unit_in_reach — D-RLSLiveReach). domain.ReachSet remains as the pure,
// property-tested reference semantic (the oracle for the reach differential test).

// FilterVisibleUnits applies the shadow-visibility gate (owned here, called via pep.FilterVisibleUnits
// by tenant's list/ancestors/descendants reads — F-002 A-lite): from candidates, drop shadow units
// the subject's *.read does not reach. `shadow` reports per unit id whether it is shadow. Returns the
// visible subset preserving input order. Since review-2026-07 R-02.1 the reach test is ONE batch SQL
// probe over only the shadow candidates (ReadableUnitsForSubjectAmong) instead of materializing the
// whole reach set app-side; public units never touch the database. domain.ShadowGate remains the pure
// reference semantic (property-tested; oracle for the reach differential test).
func (s *Service) FilterVisibleUnits(ctx context.Context, subjectPersonID string, candidates []string, shadow map[string]bool) ([]string, error) {
	shadowCandidates := make([]string, 0, len(candidates))
	for _, u := range candidates {
		if shadow[u] {
			shadowCandidates = append(shadowCandidates, u)
		}
	}
	reachable := map[string]struct{}{}
	if len(shadowCandidates) > 0 {
		isAdmin, _, err := s.authorityFor(ctx, subjectPersonID)
		if err != nil {
			return nil, err
		}
		if isAdmin {
			for _, u := range shadowCandidates {
				reachable[u] = struct{}{}
			}
		} else {
			ids, err := s.newRepo(s.pool).ReadableUnitsForSubjectAmong(ctx, subjectPersonID, shadowCandidates)
			if err != nil {
				return nil, err
			}
			for _, u := range ids {
				reachable[u] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(candidates))
	for _, u := range candidates {
		if shadow[u] {
			if _, ok := reachable[u]; !ok {
				continue
			}
		}
		out = append(out, u)
	}
	return out, nil
}

// ============================ roles ============================

// CreateRole validates and creates a custom role (is_base=false), persists its permission membership,
// and records the action. Instance-scope / unknown permission codes are rejected (domain.Validate).
func (s *Service) CreateRole(ctx context.Context, role domain.Role) (domain.Role, error) {
	role.IsBase = false
	if err := role.Validate(); err != nil {
		return domain.Role{}, err
	}
	for _, p := range role.Permissions {
		if err := s.rejectDisabledCode(p); err != nil {
			return domain.Role{}, err
		}
	}
	var out domain.Role
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		created, err := repo.InsertRole(ctx, role)
		if err != nil {
			return err
		}
		if err := repo.ReplaceRolePermissions(ctx, created.ID, role.Permissions); err != nil {
			return err
		}
		created.Permissions = role.Permissions
		out = created
		return s.record(ctx, tx, "role.create", targetRole, created.ID, map[string]any{"id": created.ID, "code": created.Code})
	})
	return out, err
}

// GetRole reads one role with its permission set.
func (s *Service) GetRole(ctx context.Context, id string) (domain.Role, error) {
	return s.newRepo(s.pool).GetRole(ctx, id)
}

// ListRoles returns a keyset-paginated page of roles (each with its permission set).
func (s *Service) ListRoles(ctx context.Context, pageSize int, pageToken string) (RolePage, error) {
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return RolePage{}, err
	}
	roles, err := s.newRepo(s.pool).ListRoles(ctx, after, size+1)
	if err != nil {
		return RolePage{}, err
	}
	if len(roles) > size {
		return RolePage{Roles: roles[:size], NextPageToken: listing.EncodeCursor(roles[size-1].ID)}, nil
	}
	return RolePage{Roles: roles}, nil
}

// UpdateRole edits a custom role's name/description and (when present) replaces its permission set.
// Base roles are immutable (ErrRoleIsBase).
func (s *Service) UpdateRole(ctx context.Context, id string, patch domain.RolePatch) (domain.Role, error) {
	if patch.Permissions != nil {
		if err := domain.ValidatePermissionSet(*patch.Permissions); err != nil {
			return domain.Role{}, err
		}
		for _, p := range *patch.Permissions {
			if err := s.rejectDisabledCode(p); err != nil {
				return domain.Role{}, err
			}
		}
	}
	var out domain.Role
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		existing, err := repo.GetRole(ctx, id)
		if err != nil {
			return err
		}
		if existing.IsBase {
			return domain.ErrRoleIsBase
		}
		updated, err := repo.UpdateRole(ctx, id, patch)
		if err != nil {
			return err
		}
		if patch.Permissions != nil {
			if err := repo.ReplaceRolePermissions(ctx, id, *patch.Permissions); err != nil {
				return err
			}
			updated.Permissions = *patch.Permissions
			// Permission-set change mutates every holder's authority — bump the revocation epoch
			// in the same tx (D-AuthzGrantCache). Name/description edits don't.
			if err := repo.BumpAuthzEpoch(ctx); err != nil {
				return err
			}
		}
		out = updated
		return s.record(ctx, tx, "role.update", targetRole, id, map[string]any{"id": id})
	})
	if err == nil && patch.Permissions != nil {
		s.grants.reset(ctx) // local cache sees the edit immediately; other replicas within the TTL
	}
	return out, err
}

// DeleteRole soft-deletes a custom role. Base roles are immutable (ErrRoleIsBase); a role with any
// active assignment is blocked (ErrRoleInUse). Orphan-translation purge is deferred (needs the event
// bus + a localization subscriber — see the module note).
func (s *Service) DeleteRole(ctx context.Context, id string) error {
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		existing, err := repo.GetRole(ctx, id)
		if err != nil {
			return err
		}
		if existing.IsBase {
			return domain.ErrRoleIsBase
		}
		inUse, err := repo.RoleHasActiveAssignments(ctx, id)
		if err != nil {
			return err
		}
		if inUse {
			return domain.ErrRoleInUse
		}
		if err := repo.SoftDeleteRole(ctx, id); err != nil {
			return err
		}
		if err := repo.BumpAuthzEpoch(ctx); err != nil {
			return err
		}
		return s.record(ctx, tx, "role.delete", targetRole, id, map[string]any{"id": id, "code": existing.Code})
	})
	if err == nil {
		s.grants.reset(ctx) // after commit: the local cache sees the delete immediately
	}
	return err
}

// ============================ assignments ============================

// GrantAssignment grants a role to a person at a target unit with unit/subtree scope. For a subtree
// grant the named graph (or the default command graph) is resolved and must be authority-bearing
// (D-DirectoryGraphs). Unless the grantor is the system (g.GrantedBy==""; bootstrap/seed), it must
// itself hold assignment.grant reaching the target unit — no self-escalation.
func (s *Service) GrantAssignment(ctx context.Context, g domain.GrantInput) (domain.Assignment, error) {
	if err := g.Validate(); err != nil {
		return domain.Assignment{}, err
	}

	var graphID string
	if g.Scope == domain.ScopeSubtree {
		id, bearing, err := s.graphs.ResolveGraph(ctx, g.TargetUnitID, g.GraphCode)
		if err != nil {
			return domain.Assignment{}, err
		}
		if !bearing {
			return domain.Assignment{}, domain.ErrNonAuthorityBearingGraph
		}
		graphID = id
	}

	if g.GrantedBy != "" { // no-self-escalation: skip only for the system/bootstrap seed path
		if err := s.Enforce(ctx, g.GrantedBy, string(domain.PermAssignmentGrant), g.TargetUnitID); err != nil {
			if errors.Is(err, domain.ErrPermissionDenied) {
				return domain.Assignment{}, domain.ErrSelfEscalation
			}
			return domain.Assignment{}, err
		}
	}

	var out domain.Assignment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		created, err := repo.InsertAssignment(ctx, g, graphID)
		if err != nil {
			return err
		}
		if err := repo.BumpAuthzEpoch(ctx); err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "assignment.grant", targetAssignment, created.ID, map[string]any{
			"id": created.ID, "subjectPersonId": created.SubjectPersonID, "roleId": created.RoleID,
			"targetUnitId": created.TargetUnitID, "scope": string(created.Scope), "graphId": created.GraphID,
		})
	})
	if err == nil {
		s.grants.reset(ctx)
	}
	return out, err
}

// RevokeAssignment flips revoked_at on an active assignment (reversible; never deleted) and records
// the action. Idempotent: revoking an already-revoked assignment returns ErrAssignmentNotFound (no
// active row matched).
func (s *Service) RevokeAssignment(ctx context.Context, id, revokedBy string) (domain.Assignment, error) {
	// No-self-escalation counterpart: a non-system revoker must hold assignment.revoke reaching the
	// assignment's target unit (skip for the system path, revokedBy=="").
	if revokedBy != "" {
		existing, err := s.newRepo(s.pool).GetAssignment(ctx, id)
		if err != nil {
			return domain.Assignment{}, err
		}
		if err := s.Enforce(ctx, revokedBy, string(domain.PermAssignmentRevoke), existing.TargetUnitID); err != nil {
			if errors.Is(err, domain.ErrPermissionDenied) {
				return domain.Assignment{}, domain.ErrSelfEscalation
			}
			return domain.Assignment{}, err
		}
	}
	var out domain.Assignment
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		revoked, err := repo.RevokeAssignment(ctx, id, revokedBy)
		if err != nil {
			return err
		}
		if err := repo.BumpAuthzEpoch(ctx); err != nil {
			return err
		}
		out = revoked
		return s.record(ctx, tx, "assignment.revoke", targetAssignment, id, map[string]any{"id": id})
	})
	if err == nil {
		s.grants.reset(ctx)
	}
	return out, err
}

// ListAssignmentsBySubject / ByUnit return keyset-paginated pages of active assignments.
func (s *Service) ListAssignmentsBySubject(ctx context.Context, subjectPersonID string, pageSize int, pageToken string) (AssignmentPage, error) {
	return s.listAssignments(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Assignment, error) {
		return s.newRepo(s.pool).ListAssignmentsBySubject(ctx, subjectPersonID, after, limit)
	})
}

func (s *Service) ListAssignmentsByUnit(ctx context.Context, targetUnitID string, pageSize int, pageToken string) (AssignmentPage, error) {
	return s.listAssignments(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Assignment, error) {
		return s.newRepo(s.pool).ListAssignmentsByUnit(ctx, targetUnitID, after, limit)
	})
}

// ============================ instance admins ============================

// GrantInstanceAdmin places a person on the instance-admin plane and records the action. The HTTP
// PEP requires instance.admin.manage (instance-scope, so only an existing instance admin) — the
// no-self-escalation invariant. grantedBy is "" for the install bootstrap (D-Bootstrap).
func (s *Service) GrantInstanceAdmin(ctx context.Context, personID, grantedBy string) (domain.InstanceAdmin, error) {
	var out domain.InstanceAdmin
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		created, err := repo.InsertInstanceAdmin(ctx, personID, grantedBy)
		if err != nil {
			return err
		}
		if err := repo.BumpAuthzEpoch(ctx); err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "instance.admin.grant", targetInstanceAdmin, created.ID, map[string]any{"id": created.ID, "personId": personID})
	})
	if err == nil {
		s.grants.reset(ctx)
	}
	return out, err
}

// RevokeInstanceAdmin removes a person from the instance-admin plane (reversible flip).
func (s *Service) RevokeInstanceAdmin(ctx context.Context, id, revokedBy string) (domain.InstanceAdmin, error) {
	var out domain.InstanceAdmin
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		revoked, err := repo.RevokeInstanceAdmin(ctx, id, revokedBy)
		if err != nil {
			return err
		}
		if err := repo.BumpAuthzEpoch(ctx); err != nil {
			return err
		}
		out = revoked
		return s.record(ctx, tx, "instance.admin.revoke", targetInstanceAdmin, id, map[string]any{"id": id})
	})
	if err == nil {
		s.grants.reset(ctx)
	}
	return out, err
}

// IsInstanceAdmin reports whether a person is on the instance plane (used by PEPs for instance-scope
// gates and by person/document read-scope resolution — D-PersonReadScope).
func (s *Service) IsInstanceAdmin(ctx context.Context, personID string) (bool, error) {
	return s.newRepo(s.pool).IsActiveInstanceAdmin(ctx, personID)
}

// ============================ seeding ============================

// SeedBaseRoles idempotently seeds the four base roles (D-BaseRoles) and re-syncs their code-defined
// permission sets on every boot, on the GUC-bearing pool (RID-keyed Objects — D-RIDSeeding). This is
// a boot-time infrastructure seed (like the tenant graph registry), not an audited domain mutation.
func (s *Service) SeedBaseRoles(ctx context.Context) error {
	for _, br := range domain.BaseRoles() {
		err := s.inTx(ctx, func(tx pgx.Tx) error {
			repo := s.newRepo(tx)
			role, err := repo.GetRoleByCode(ctx, br.Code)
			if errors.Is(err, domain.ErrRoleNotFound) {
				role, err = repo.InsertRole(ctx, domain.Role{Code: br.Code, Name: br.Name, Description: br.Description, IsBase: true})
			}
			if err != nil {
				return err
			}
			if err := repo.ReplaceRolePermissions(ctx, role.ID, br.Permissions); err != nil {
				return err
			}
			// The permission re-sync can change a base role's set across versions — bump the
			// revocation epoch so caches on other replicas converge within the TTL.
			return repo.BumpAuthzEpoch(ctx)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// ============================ helpers ============================

func (s *Service) listAssignments(ctx context.Context, pageSize int, pageToken string, fetch func(after string, limit int) ([]domain.Assignment, error)) (AssignmentPage, error) {
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return AssignmentPage{}, err
	}
	as, err := fetch(after, size+1)
	if err != nil {
		return AssignmentPage{}, err
	}
	if len(as) > size {
		return AssignmentPage{Assignments: as[:size], NextPageToken: listing.EncodeCursor(as[size-1].ID)}, nil
	}
	return AssignmentPage{Assignments: as}, nil
}

// inTx runs fn in a transaction, committing on success and rolling back on error.
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the
// audit entry commits iff the change commits (D-Audit). The actor is the interim system actor.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID mints an Action RID (authz service=8, kind=action=3, generic action type=0) via the
// shared SQL generator, so the audit log's rid_kind=3 shape CHECK is satisfied. The specific action
// name is recorded separately in audit_log.action (D-Audit), so the RID need not encode it.
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	_ = action
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(8, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

// requestID is the correlation key shared with logs/metrics/traces: the request's trace id, with a
// generated fallback for out-of-request callers (e.g. integration tests).
func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
