// Package transport implements the generated Conjure AuthorizationService interface: it translates
// the wire contract to/from the application service, assembles localized role name/description maps
// via the localization service (cross-module query — overview.md), enforces the endpoints' own
// permissions through the PEP, and maps domain errors to Conjure SerializableErrors (D-Conjure).
// Generated code in internal/conjure is never hand-edited.
//
// Permission gates (docs/modules/authorization.md): /authorize[/batch] require assignment.read
// reaching the queried unit (no self-exemption, OQ-5); role.create/update/delete are instance-scope;
// role reads require role.read held anywhere (roles are instance-global, not unit-keyed);
// assignment.grant/revoke are checked in the application against the target unit; instance.admin.manage
// is instance-scope. The bearer token carries the acting subject (interim: token == person RID).
package transport

import (
	"context"
	"errors"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/authorization/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	authzapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/authorization"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// roleEntity is the localization entity-type key for role labels; descriptionField is the i18n field
// the translatable description lives under (name uses the default "name" field via NamesByID).
const (
	roleEntity       = "role"
	descriptionField = "description"
)

// Service adapts *application.Service to the generated AuthorizationService interface. It holds the
// localization service (role name/description maps) and the PEP (endpoint permission gates).
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the authorization application service, the
// localization service, and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ authzapi.AuthorizationService = Service{}

// ---------------------------------------------------------------- decisions

func (s Service) Authorize(ctx context.Context, token bearertoken.Token, req authzapi.AuthorizeRequest) (authzapi.AuthorizeResponse, error) {
	unitID := derefOr(req.UnitId, "")
	if err := s.pep.Require(ctx, token, string(domain.PermAssignmentRead), unitID); err != nil {
		return authzapi.AuthorizeResponse{}, err
	}
	d, err := s.app.Decide(ctx, req.SubjectPersonId, req.Action, unitID, derefOr(req.Explain, false))
	if err != nil {
		return authzapi.AuthorizeResponse{}, s.mapError(ctx, err)
	}
	return toAPIDecision(d, derefOr(req.Explain, false)), nil
}

func (s Service) AuthorizeBatch(ctx context.Context, token bearertoken.Token, req authzapi.BatchAuthorizeRequest) (authzapi.BatchAuthorizeResponse, error) {
	explain := derefOr(req.Explain, false)
	// Gate on assignment.read reaching every queried unit (no self-exemption); deny the whole batch
	// if the caller cannot ask about any one of them.
	queries := make([]application.BatchQuery, 0, len(req.Queries))
	for _, q := range req.Queries {
		unitID := derefOr(q.UnitId, "")
		if err := s.pep.Require(ctx, token, string(domain.PermAssignmentRead), unitID); err != nil {
			return authzapi.BatchAuthorizeResponse{}, err
		}
		queries = append(queries, application.BatchQuery{Action: q.Action, UnitID: unitID})
	}
	decisions, err := s.app.DecideBatch(ctx, req.SubjectPersonId, queries, explain)
	if err != nil {
		return authzapi.BatchAuthorizeResponse{}, s.mapError(ctx, err)
	}
	out := make([]authzapi.AuthorizeResponse, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, toAPIDecision(d, explain))
	}
	return authzapi.BatchAuthorizeResponse{Decisions: out}, nil
}

// ---------------------------------------------------------------- roles

func (s Service) CreateRole(ctx context.Context, token bearertoken.Token, req authzapi.CreateRoleRequest) (authzapi.Role, error) {
	if err := s.pep.Require(ctx, token, string(domain.PermRoleCreate), ""); err != nil {
		return authzapi.Role{}, err
	}
	created, err := s.app.CreateRole(ctx, domain.Role{
		Code:        req.Code,
		Name:        req.Name,
		Description: derefOr(req.Description, ""),
		Permissions: toPerms(req.Permissions),
	})
	if err != nil {
		return authzapi.Role{}, s.mapError(ctx, err)
	}
	return s.roleToAPI(ctx, created)
}

func (s Service) ListRoles(ctx context.Context, token bearertoken.Token, pageSize *int, pageToken *string) (authzapi.RolePage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(domain.PermRoleRead)); err != nil {
		return authzapi.RolePage{}, err
	}
	page, err := s.app.ListRoles(ctx, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return authzapi.RolePage{}, s.mapError(ctx, err)
	}
	roles, err := s.rolesToAPI(ctx, page.Roles)
	if err != nil {
		return authzapi.RolePage{}, err
	}
	return authzapi.RolePage{Roles: roles, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

func (s Service) GetRole(ctx context.Context, token bearertoken.Token, roleID string) (authzapi.Role, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(domain.PermRoleRead)); err != nil {
		return authzapi.Role{}, err
	}
	role, err := s.app.GetRole(ctx, roleID)
	if err != nil {
		return authzapi.Role{}, s.mapError(ctx, err)
	}
	return s.roleToAPI(ctx, role)
}

func (s Service) UpdateRole(ctx context.Context, token bearertoken.Token, roleID string, req authzapi.UpdateRoleRequest) (authzapi.Role, error) {
	if err := s.pep.Require(ctx, token, string(domain.PermRoleUpdate), ""); err != nil {
		return authzapi.Role{}, err
	}
	patch := domain.RolePatch{Name: req.Name, Description: req.Description}
	if req.Permissions != nil {
		perms := toPerms(*req.Permissions)
		patch.Permissions = &perms
	}
	updated, err := s.app.UpdateRole(ctx, roleID, patch)
	if err != nil {
		return authzapi.Role{}, s.mapError(ctx, err)
	}
	return s.roleToAPI(ctx, updated)
}

func (s Service) DeleteRole(ctx context.Context, token bearertoken.Token, roleID string) error {
	if err := s.pep.Require(ctx, token, string(domain.PermRoleDelete), ""); err != nil {
		return err
	}
	if err := s.app.DeleteRole(ctx, roleID); err != nil {
		return s.mapError(ctx, err)
	}
	return nil
}

// ---------------------------------------------------------------- assignments

func (s Service) GrantAssignment(ctx context.Context, token bearertoken.Token, req authzapi.GrantAssignmentRequest) (authzapi.Assignment, error) {
	// The grant's own authority check (assignment.grant reaching the target unit) is enforced in the
	// application against the resolved grantor; here we just carry the acting subject through.
	created, err := s.app.GrantAssignment(ctx, domain.GrantInput{
		SubjectPersonID: req.SubjectPersonId,
		RoleID:          req.RoleId,
		TargetUnitID:    req.TargetUnitId,
		Scope:           domain.Scope(req.Scope),
		GraphCode:       derefOr(req.Graph, ""),
		GrantedBy:       pep.Subject(ctx),
		ExpiresAt:       dtPtr(req.ExpiresAt),
	})
	if err != nil {
		return authzapi.Assignment{}, s.mapError(ctx, err)
	}
	return toAPIAssignment(created), nil
}

func (s Service) RevokeAssignment(ctx context.Context, token bearertoken.Token, assignmentID string) (authzapi.Assignment, error) {
	revoked, err := s.app.RevokeAssignment(ctx, assignmentID, pep.Subject(ctx))
	if err != nil {
		return authzapi.Assignment{}, s.mapError(ctx, err)
	}
	return toAPIAssignment(revoked), nil
}

func (s Service) ListAssignments(ctx context.Context, token bearertoken.Token, subjectPersonID *string, targetUnitID *string, pageSize *int, pageToken *string) (authzapi.AssignmentPage, error) {
	subj := derefOr(subjectPersonID, "")
	unit := derefOr(targetUnitID, "")
	switch {
	case unit != "" && subj == "":
		if err := s.pep.Require(ctx, token, string(domain.PermAssignmentRead), unit); err != nil {
			return authzapi.AssignmentPage{}, err
		}
		page, err := s.app.ListAssignmentsByUnit(ctx, unit, derefOr(pageSize, 0), derefOr(pageToken, ""))
		if err != nil {
			return authzapi.AssignmentPage{}, s.mapError(ctx, err)
		}
		return toAPIAssignmentPage(page), nil
	case subj != "" && unit == "":
		if err := s.pep.RequireAnywhere(ctx, token, string(domain.PermAssignmentRead)); err != nil {
			return authzapi.AssignmentPage{}, err
		}
		page, err := s.app.ListAssignmentsBySubject(ctx, subj, derefOr(pageSize, 0), derefOr(pageToken, ""))
		if err != nil {
			return authzapi.AssignmentPage{}, s.mapError(ctx, err)
		}
		return toAPIAssignmentPage(page), nil
	default:
		return authzapi.AssignmentPage{}, authzapi.NewAssignmentInvalid("provide exactly one of subjectPersonId or targetUnitId")
	}
}

// ---------------------------------------------------------------- instance admins

func (s Service) GrantInstanceAdmin(ctx context.Context, token bearertoken.Token, req authzapi.GrantInstanceAdminRequest) (authzapi.InstanceAdmin, error) {
	if err := s.pep.Require(ctx, token, string(domain.PermInstanceAdminManage), ""); err != nil {
		return authzapi.InstanceAdmin{}, err
	}
	created, err := s.app.GrantInstanceAdmin(ctx, req.PersonId, pep.Subject(ctx))
	if err != nil {
		return authzapi.InstanceAdmin{}, s.mapError(ctx, err)
	}
	return toAPIInstanceAdmin(created), nil
}

func (s Service) RevokeInstanceAdmin(ctx context.Context, token bearertoken.Token, instanceAdminID string) (authzapi.InstanceAdmin, error) {
	if err := s.pep.Require(ctx, token, string(domain.PermInstanceAdminManage), ""); err != nil {
		return authzapi.InstanceAdmin{}, err
	}
	revoked, err := s.app.RevokeInstanceAdmin(ctx, instanceAdminID, pep.Subject(ctx))
	if err != nil {
		return authzapi.InstanceAdmin{}, s.mapError(ctx, err)
	}
	return toAPIInstanceAdmin(revoked), nil
}

// ---------------------------------------------------------------- response assembly

func (s Service) roleToAPI(ctx context.Context, r domain.Role) (authzapi.Role, error) {
	names, err := s.loc.NamesByID(ctx, roleEntity, map[string]string{r.ID: r.Name})
	if err != nil {
		return authzapi.Role{}, err
	}
	descs, err := s.loc.LabelsByID(ctx, roleEntity, descriptionField, map[string]string{r.ID: r.Description})
	if err != nil {
		return authzapi.Role{}, err
	}
	return toAPIRole(r, names[r.ID], descs[r.ID]), nil
}

func (s Service) rolesToAPI(ctx context.Context, roles []domain.Role) ([]authzapi.Role, error) {
	nameDefaults := make(map[string]string, len(roles))
	descDefaults := make(map[string]string, len(roles))
	for _, r := range roles {
		nameDefaults[r.ID] = r.Name
		descDefaults[r.ID] = r.Description
	}
	names, err := s.loc.NamesByID(ctx, roleEntity, nameDefaults)
	if err != nil {
		return nil, err
	}
	descs, err := s.loc.LabelsByID(ctx, roleEntity, descriptionField, descDefaults)
	if err != nil {
		return nil, err
	}
	out := make([]authzapi.Role, 0, len(roles))
	for _, r := range roles {
		out = append(out, toAPIRole(r, names[r.ID], descs[r.ID]))
	}
	return out, nil
}

func toAPIRole(r domain.Role, name, desc map[string]string) authzapi.Role {
	perms := make([]string, 0, len(r.Permissions))
	for _, p := range r.Permissions {
		perms = append(perms, string(p))
	}
	return authzapi.Role{
		Id:          r.ID,
		Code:        r.Code,
		Name:        name,
		Description: desc,
		Permissions: perms,
		IsBase:      r.IsBase,
		CreatedAt:   datetime.DateTime(r.CreatedAt),
		UpdatedAt:   datetime.DateTime(r.UpdatedAt),
	}
}

func toAPIAssignment(a domain.Assignment) authzapi.Assignment {
	return authzapi.Assignment{
		Id:              a.ID,
		SubjectPersonId: a.SubjectPersonID,
		RoleId:          a.RoleID,
		TargetUnitId:    a.TargetUnitID,
		Scope:           string(a.Scope),
		GraphId:         strPtrOrNil(a.GraphID),
		GrantedBy:       strPtrOrNil(a.GrantedBy),
		GrantedAt:       datetime.DateTime(a.GrantedAt),
		RevokedAt:       dtPtrOf(a.RevokedAt),
		RevokedBy:       strPtrOrNil(a.RevokedBy),
		ExpiresAt:       dtPtrOf(a.ExpiresAt),
		CreatedAt:       datetime.DateTime(a.CreatedAt),
		UpdatedAt:       datetime.DateTime(a.UpdatedAt),
	}
}

func toAPIAssignmentPage(page application.AssignmentPage) authzapi.AssignmentPage {
	as := make([]authzapi.Assignment, 0, len(page.Assignments))
	for _, a := range page.Assignments {
		as = append(as, toAPIAssignment(a))
	}
	return authzapi.AssignmentPage{Assignments: as, NextPageToken: tokenPtr(page.NextPageToken)}
}

func toAPIInstanceAdmin(a domain.InstanceAdmin) authzapi.InstanceAdmin {
	return authzapi.InstanceAdmin{
		Id:        a.ID,
		PersonId:  a.PersonID,
		GrantedBy: strPtrOrNil(a.GrantedBy),
		GrantedAt: datetime.DateTime(a.GrantedAt),
		RevokedAt: dtPtrOf(a.RevokedAt),
		RevokedBy: strPtrOrNil(a.RevokedBy),
	}
}

func toAPIDecision(d domain.Decision, explain bool) authzapi.AuthorizeResponse {
	resp := authzapi.AuthorizeResponse{Allow: d.Allow}
	if explain {
		ex := authzapi.Explanation{Contributions: make([]authzapi.Contribution, 0, len(d.Via))}
		for _, c := range d.Via {
			ex.Contributions = append(ex.Contributions, authzapi.Contribution{
				InstanceAdmin: c.InstanceAdmin,
				AssignmentId:  strPtrOrNil(c.AssignmentID),
				RoleCode:      strPtrOrNil(c.RoleCode),
				TargetUnitId:  strPtrOrNil(c.TargetUnitID),
				Scope:         strPtrOrNil(string(c.Scope)),
				GraphCode:     strPtrOrNil(c.GraphCode),
			})
		}
		if d.DenyReason != "" {
			ex.DenyReason = &d.DenyReason
		}
		resp.Explanation = &ex
	}
	return resp
}

// ---------------------------------------------------------------- error mapping

func (s Service) mapError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrRoleNotFound):
		return authzapi.NewRoleNotFound("")
	case errors.Is(err, domain.ErrRoleCodeConflict):
		return authzapi.NewRoleConflict("a role with this code already exists")
	case errors.Is(err, domain.ErrRoleIsBase):
		return authzapi.NewRoleImmutable("")
	case errors.Is(err, domain.ErrRoleInUse):
		return authzapi.NewRoleInUse("")
	case errors.Is(err, domain.ErrUnknownPermission):
		return authzapi.NewRoleInvalid(err.Error())
	case errors.Is(err, domain.ErrRoleInvalid):
		return authzapi.NewRoleInvalid(err.Error())
	case errors.Is(err, domain.ErrAssignmentNotFound):
		return authzapi.NewAssignmentNotFound("")
	case errors.Is(err, domain.ErrAssignmentConflict):
		return authzapi.NewAssignmentConflict("an identical active assignment already exists")
	case errors.Is(err, domain.ErrNonAuthorityBearingGraph):
		return authzapi.NewNonAuthorityBearingGraph("")
	case errors.Is(err, domain.ErrSelfEscalation):
		return authzapi.NewSelfEscalation("grantor lacks authority to grant/revoke this assignment")
	case errors.Is(err, domain.ErrUnknownSubject):
		return authzapi.NewAssignmentInvalid("subject person does not exist")
	case errors.Is(err, domain.ErrUnknownRole):
		return authzapi.NewAssignmentInvalid("role does not exist")
	case errors.Is(err, domain.ErrUnknownUnit):
		return authzapi.NewAssignmentInvalid("target unit does not exist")
	case errors.Is(err, domain.ErrUnknownGraph):
		return authzapi.NewAssignmentInvalid("graph does not exist")
	case errors.Is(err, domain.ErrAssignmentInvalid):
		return authzapi.NewAssignmentInvalid(err.Error())
	case errors.Is(err, domain.ErrInstanceAdminNotFound):
		return authzapi.NewInstanceAdminNotFound("")
	case errors.Is(err, domain.ErrInstanceAdminConflict):
		return authzapi.NewInstanceAdminConflict("the person is already an active instance admin")
	case errors.Is(err, domain.ErrPermissionDenied):
		return authzapi.NewPermissionDenied("")
	default:
		return werror.WrapWithContextParams(ctx, err, "authorization request failed")
	}
}

// ---------------------------------------------------------------- value helpers

func toPerms(codes []string) []domain.Permission {
	out := make([]domain.Permission, 0, len(codes))
	for _, c := range codes {
		out = append(out, domain.Permission(c))
	}
	return out
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func tokenPtr(token string) *string {
	if token == "" {
		return nil
	}
	return &token
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

func dtPtr(d *datetime.DateTime) *time.Time {
	if d == nil {
		return nil
	}
	t := time.Time(*d)
	return &t
}

func dtPtrOf(t *time.Time) *datetime.DateTime {
	if t == nil {
		return nil
	}
	d := datetime.DateTime(*t)
	return &d
}
