// Package transport implements the tenant module's generated Conjure TenantService interface: it
// translates the wire contract to/from the application service, assembles localized `name` maps via
// the localization service (cross-module query — overview.md), and maps domain errors to Conjure
// SerializableErrors (D-Conjure). Generated code in internal/conjure is never hand-edited.
//
// Authorization (M7): unit endpoints gate on their unit-scoped permission AT the unit via the PEP
// (read/update/lifecycle); edge mutations require the per-graph OR broad edge permission at the path
// unit (D-EdgePerms); creating a unit and reading the graph registry use the coarse "holds anywhere"
// form (a unit is created standalone, with no parent to scope against — root creation falls to the
// instance admin); graph management and on-demand closure verify/rebuild are instance-scope. The
// shadow-visibility gate on list/ancestor/descendant reads is a documented follow-up. The bearer
// token carries the acting subject (interim: token == person RID; see internal/authorization/pep).
package transport

import (
	"context"
	"encoding/json"
	"errors"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	tenantapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/tenant"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// Service adapts *application.Service to the generated tenantapi.TenantService interface. It holds
// the localization service to assemble the `locale -> text` display-name maps responses return, and
// the PEP enforcer for the endpoints' permission gates.
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the tenant application service, the localization
// service (for name-map assembly), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

// edgeActions returns the acceptable edge-management permissions for a graph: the per-graph code (if
// one exists for that graph; only command/operational do) plus the broad fallback (D-EdgePerms).
func edgeActions(graph string) []string {
	perGraph := "unit.edges." + graph + ".manage"
	return []string{perGraph, string(authzdomain.PermUnitEdgesManage)}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ tenantapi.TenantService = Service{}

// ---------------------------------------------------------------- units

func (s Service) CreateUnit(ctx context.Context, token bearertoken.Token, req tenantapi.CreateUnitRequest) (tenantapi.Unit, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermUnitCreate)); err != nil {
		return tenantapi.Unit{}, err
	}
	u := domain.Unit{
		Code:       req.Code,
		Name:       req.Name,
		UnitKind:   derefOr(req.UnitKind, ""),
		Level:      req.Level,
		Visibility: visibilityOrDefault(req.Visibility),
		Metadata:   rawFromAny(req.Metadata),
	}
	created, err := s.app.CreateUnit(ctx, u)
	if err != nil {
		return tenantapi.Unit{}, s.mapError(ctx, err, errCtx{code: req.Code})
	}
	return s.unitToAPI(ctx, created)
}

func (s Service) GetUnit(ctx context.Context, token bearertoken.Token, unitID string) (tenantapi.Unit, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitRead), unitID); err != nil {
		return tenantapi.Unit{}, err
	}
	u, err := s.app.GetUnit(ctx, unitID)
	if err != nil {
		return tenantapi.Unit{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return s.unitToAPI(ctx, u)
}

func (s Service) UpdateUnit(ctx context.Context, token bearertoken.Token, unitID string, req tenantapi.UpdateUnitRequest) (tenantapi.Unit, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitUpdate), unitID); err != nil {
		return tenantapi.Unit{}, err
	}
	patch := domain.UnitPatch{
		Name:     req.Name,
		UnitKind: req.UnitKind,
		Level:    req.Level,
		Metadata: rawFromAny(req.Metadata),
	}
	if req.Visibility != nil {
		v := fromAPIVisibility(*req.Visibility)
		patch.Visibility = &v
	}
	updated, err := s.app.UpdateUnit(ctx, unitID, patch)
	if err != nil {
		return tenantapi.Unit{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return s.unitToAPI(ctx, updated)
}

func (s Service) ListUnits(ctx context.Context, token bearertoken.Token, level *int, pageSize *int, pageToken *string) (tenantapi.UnitPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermUnitRead)); err != nil {
		return tenantapi.UnitPage{}, err
	}
	page, err := s.app.ListUnits(ctx, level, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return tenantapi.UnitPage{}, s.mapError(ctx, err, errCtx{})
	}
	units, err := s.unitsToAPI(ctx, page.Units)
	if err != nil {
		return tenantapi.UnitPage{}, s.mapError(ctx, err, errCtx{})
	}
	return tenantapi.UnitPage{Units: units, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

func (s Service) TransitionUnit(ctx context.Context, token bearertoken.Token, unitID string, req tenantapi.TransitionRequest) (tenantapi.Unit, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitLifecycle), unitID); err != nil {
		return tenantapi.Unit{}, err
	}
	updated, err := s.app.TransitionUnit(ctx, unitID, fromAPIState(req.ToState), derefOr(req.Reason, ""))
	if err != nil {
		return tenantapi.Unit{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return s.unitToAPI(ctx, updated)
}

// ---------------------------------------------------------------- edges

func (s Service) AddEdge(ctx context.Context, token bearertoken.Token, unitID string, req tenantapi.AddEdgeRequest) (tenantapi.UnitEdge, error) {
	graph := derefOr(req.Graph, domain.CommandGraphCode)
	if err := s.pep.RequireAny(ctx, token, unitID, edgeActions(graph)...); err != nil {
		return tenantapi.UnitEdge{}, err
	}
	edge, err := s.app.AddEdge(ctx, unitID, req.ParentId, graph)
	if err != nil {
		return tenantapi.UnitEdge{}, s.mapError(ctx, err, errCtx{
			unitID: unitID, graph: graph, parentID: req.ParentId, childID: unitID,
		})
	}
	return tenantapi.UnitEdge{
		Id:        edge.ID,
		Graph:     edge.Graph,
		ParentId:  edge.ParentID,
		ChildId:   edge.ChildID,
		CreatedAt: datetime.DateTime(edge.CreatedAt),
	}, nil
}

func (s Service) RemoveEdge(ctx context.Context, token bearertoken.Token, unitID string, parentID string, graph *string) error {
	g := derefOr(graph, domain.CommandGraphCode)
	if err := s.pep.RequireAny(ctx, token, unitID, edgeActions(g)...); err != nil {
		return err
	}
	if err := s.app.RemoveEdge(ctx, unitID, parentID, g); err != nil {
		return s.mapError(ctx, err, errCtx{unitID: unitID, graph: g, parentID: parentID, childID: unitID})
	}
	return nil
}

func (s Service) UnitAncestors(ctx context.Context, token bearertoken.Token, unitID string, graph *string) (tenantapi.UnitRefList, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitRead), unitID); err != nil {
		return tenantapi.UnitRefList{}, err
	}
	refs, err := s.app.Ancestors(ctx, unitID, derefOr(graph, domain.CommandGraphCode))
	if err != nil {
		return tenantapi.UnitRefList{}, s.mapError(ctx, err, errCtx{unitID: unitID, graph: derefOr(graph, domain.CommandGraphCode)})
	}
	out, err := s.refsToAPI(ctx, refs)
	if err != nil {
		return tenantapi.UnitRefList{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return tenantapi.UnitRefList{Units: out}, nil
}

func (s Service) UnitDescendants(ctx context.Context, token bearertoken.Token, unitID string, graph *string, pageSize *int, pageToken *string) (tenantapi.UnitRefPage, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitRead), unitID); err != nil {
		return tenantapi.UnitRefPage{}, err
	}
	page, err := s.app.Descendants(ctx, unitID, derefOr(graph, domain.CommandGraphCode), derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return tenantapi.UnitRefPage{}, s.mapError(ctx, err, errCtx{unitID: unitID, graph: derefOr(graph, domain.CommandGraphCode)})
	}
	out, err := s.refsToAPI(ctx, page.Refs)
	if err != nil {
		return tenantapi.UnitRefPage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return tenantapi.UnitRefPage{Units: out, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

// ---------------------------------------------------------------- closure

func (s Service) VerifyClosure(ctx context.Context, token bearertoken.Token, graph *string) (tenantapi.ClosureReportList, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermClosureRebuild), ""); err != nil {
		return tenantapi.ClosureReportList{}, err
	}
	reports, err := s.app.VerifyClosure(ctx, graph)
	if err != nil {
		return tenantapi.ClosureReportList{}, s.mapError(ctx, err, errCtx{graph: derefOr(graph, "")})
	}
	return tenantapi.ClosureReportList{Reports: toAPIReports(reports)}, nil
}

func (s Service) RebuildClosure(ctx context.Context, token bearertoken.Token, graph *string) (tenantapi.ClosureReportList, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermClosureRebuild), ""); err != nil {
		return tenantapi.ClosureReportList{}, err
	}
	reports, err := s.app.RebuildClosure(ctx, graph)
	if err != nil {
		return tenantapi.ClosureReportList{}, s.mapError(ctx, err, errCtx{graph: derefOr(graph, "")})
	}
	return tenantapi.ClosureReportList{Reports: toAPIReports(reports)}, nil
}

// ---------------------------------------------------------------- graphs

func (s Service) ListGraphs(ctx context.Context, token bearertoken.Token) (tenantapi.GraphList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermGraphRead)); err != nil {
		return tenantapi.GraphList{}, err
	}
	graphs, err := s.app.ListGraphs(ctx)
	if err != nil {
		return tenantapi.GraphList{}, s.mapError(ctx, err, errCtx{})
	}
	out, err := s.graphsToAPI(ctx, graphs)
	if err != nil {
		return tenantapi.GraphList{}, s.mapError(ctx, err, errCtx{})
	}
	return tenantapi.GraphList{Graphs: out}, nil
}

func (s Service) AddGraph(ctx context.Context, token bearertoken.Token, req tenantapi.AddGraphRequest) (tenantapi.Graph, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermGraphManage), ""); err != nil {
		return tenantapi.Graph{}, err
	}
	created, err := s.app.AddGraph(ctx, req.Code, req.Name, derefOr(req.IsAuthorityBearing, true))
	if err != nil {
		return tenantapi.Graph{}, s.mapError(ctx, err, errCtx{code: req.Code})
	}
	return s.graphToAPI(ctx, created)
}

func (s Service) UpdateGraph(ctx context.Context, token bearertoken.Token, graphID string, req tenantapi.UpdateGraphRequest) (tenantapi.Graph, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermGraphManage), ""); err != nil {
		return tenantapi.Graph{}, err
	}
	updated, err := s.app.UpdateGraph(ctx, graphID, domain.GraphPatch{
		Name:               req.Name,
		IsDefault:          req.IsDefault,
		IsAuthorityBearing: req.IsAuthorityBearing,
	})
	if err != nil {
		return tenantapi.Graph{}, s.mapError(ctx, err, errCtx{graph: graphID})
	}
	return s.graphToAPI(ctx, updated)
}

func (s Service) DeleteGraph(ctx context.Context, token bearertoken.Token, graphID string) error {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermGraphManage), ""); err != nil {
		return err
	}
	if err := s.app.DeleteGraph(ctx, graphID); err != nil {
		return s.mapError(ctx, err, errCtx{graph: graphID})
	}
	return nil
}

// ---------------------------------------------------------------- response assembly

func (s Service) unitToAPI(ctx context.Context, u domain.Unit) (tenantapi.Unit, error) {
	names, err := s.loc.NamesByID(ctx, "unit", map[string]string{u.ID: u.Name})
	if err != nil {
		return tenantapi.Unit{}, err
	}
	return toAPIUnit(u, names[u.ID]), nil
}

func (s Service) unitsToAPI(ctx context.Context, units []domain.Unit) ([]tenantapi.Unit, error) {
	defaults := make(map[string]string, len(units))
	for _, u := range units {
		defaults[u.ID] = u.Name
	}
	names, err := s.loc.NamesByID(ctx, "unit", defaults)
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.Unit, 0, len(units))
	for _, u := range units {
		out = append(out, toAPIUnit(u, names[u.ID]))
	}
	return out, nil
}

func (s Service) refsToAPI(ctx context.Context, refs []domain.UnitRef) ([]tenantapi.UnitRef, error) {
	defaults := make(map[string]string, len(refs))
	for _, r := range refs {
		defaults[r.ID] = r.Name
	}
	names, err := s.loc.NamesByID(ctx, "unit", defaults)
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.UnitRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, tenantapi.UnitRef{Id: r.ID, Code: r.Code, Name: names[r.ID], Depth: r.Depth})
	}
	return out, nil
}

func (s Service) graphToAPI(ctx context.Context, g domain.Graph) (tenantapi.Graph, error) {
	names, err := s.loc.NamesByID(ctx, "graph", map[string]string{g.ID: g.Name})
	if err != nil {
		return tenantapi.Graph{}, err
	}
	return toAPIGraph(g, names[g.ID]), nil
}

func (s Service) graphsToAPI(ctx context.Context, graphs []domain.Graph) ([]tenantapi.Graph, error) {
	defaults := make(map[string]string, len(graphs))
	for _, g := range graphs {
		defaults[g.ID] = g.Name
	}
	names, err := s.loc.NamesByID(ctx, "graph", defaults)
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.Graph, 0, len(graphs))
	for _, g := range graphs {
		out = append(out, toAPIGraph(g, names[g.ID]))
	}
	return out, nil
}

func toAPIUnit(u domain.Unit, name map[string]string) tenantapi.Unit {
	return tenantapi.Unit{
		Id:         u.ID,
		Code:       u.Code,
		Name:       name,
		UnitKind:   strPtrOrNil(u.UnitKind),
		Level:      u.Level,
		Visibility: toAPIVisibility(u.Visibility),
		State:      toAPIState(u.State),
		Metadata:   anyFromRaw(u.Metadata),
		CreatedAt:  datetime.DateTime(u.CreatedAt),
		UpdatedAt:  datetime.DateTime(u.UpdatedAt),
	}
}

func toAPIGraph(g domain.Graph, name map[string]string) tenantapi.Graph {
	return tenantapi.Graph{
		Id:                 g.ID,
		Code:               g.Code,
		Name:               name,
		IsDefault:          g.IsDefault,
		IsAuthorityBearing: g.IsAuthorityBearing,
	}
}

func toAPIReports(reports []domain.ClosureReport) []tenantapi.ClosureReport {
	out := make([]tenantapi.ClosureReport, 0, len(reports))
	for _, r := range reports {
		out = append(out, tenantapi.ClosureReport{
			Graph:        r.Graph,
			MissingCount: r.MissingCount,
			ExtraCount:   r.ExtraCount,
			InDrift:      r.InDrift,
			Sample:       anyFromRaw(r.Sample),
		})
	}
	return out
}

// ---------------------------------------------------------------- error mapping

// errCtx carries the identifiers an endpoint can name in a Conjure error (only the relevant fields
// are set per call).
type errCtx struct {
	unitID   string
	code     string
	graph    string
	parentID string
	childID  string
}

// mapError translates domain/application errors into the Conjure SerializableError contract.
func (s Service) mapError(ctx context.Context, err error, c errCtx) error {
	switch {
	case errors.Is(err, domain.ErrUnitNotFound):
		return tenantapi.NewUnitNotFound(c.unitID)
	case errors.Is(err, domain.ErrUnitCodeConflict):
		return tenantapi.NewUnitCodeConflict(c.code)
	case errors.Is(err, domain.ErrUnitCycle):
		return tenantapi.NewUnitCycleDetected(c.graph, c.parentID, c.childID)
	case errors.Is(err, domain.ErrEdgeExists):
		return tenantapi.NewUnitInvalid("edge already exists in graph " + c.graph)
	case errors.Is(err, domain.ErrInvalidTransition):
		return tenantapi.NewTransitionInvalid(err.Error())
	case errors.Is(err, domain.ErrInvalidUnit):
		return tenantapi.NewUnitInvalid(err.Error())
	case errors.Is(err, domain.ErrGraphNotFound):
		return tenantapi.NewGraphNotFound(c.graph)
	case errors.Is(err, domain.ErrGraphCodeConflict):
		return tenantapi.NewGraphCodeConflict(c.code)
	case errors.Is(err, domain.ErrGraphInUse):
		return tenantapi.NewGraphInUse(c.graph)
	case errors.Is(err, domain.ErrGraphProtected):
		return tenantapi.NewGraphProtected(err.Error())
	default:
		return werror.WrapWithContextParams(ctx, err, "tenant request failed")
	}
}

// ---------------------------------------------------------------- enum / value helpers

func toAPIVisibility(v domain.Visibility) tenantapi.Visibility {
	if v == domain.VisibilityShadow {
		return tenantapi.New_Visibility(tenantapi.Visibility_SHADOW)
	}
	return tenantapi.New_Visibility(tenantapi.Visibility_PUBLIC)
}

func fromAPIVisibility(v tenantapi.Visibility) domain.Visibility {
	if v.Value() == tenantapi.Visibility_SHADOW {
		return domain.VisibilityShadow
	}
	return domain.VisibilityPublic
}

func visibilityOrDefault(v *tenantapi.Visibility) domain.Visibility {
	if v == nil {
		return domain.VisibilityPublic
	}
	return fromAPIVisibility(*v)
}

func toAPIState(s domain.State) tenantapi.UnitState {
	switch s {
	case domain.StateSuspended:
		return tenantapi.New_UnitState(tenantapi.UnitState_SUSPENDED)
	case domain.StateArchived:
		return tenantapi.New_UnitState(tenantapi.UnitState_ARCHIVED)
	default:
		return tenantapi.New_UnitState(tenantapi.UnitState_ACTIVE)
	}
}

func fromAPIState(s tenantapi.UnitState) domain.State {
	switch s.Value() {
	case tenantapi.UnitState_SUSPENDED:
		return domain.StateSuspended
	case tenantapi.UnitState_ARCHIVED:
		return domain.StateArchived
	default:
		return domain.StateActive
	}
}

// anyFromRaw decodes a JSONB raw message into the Conjure `any` (*interface{}); nil/empty -> nil.
func anyFromRaw(raw json.RawMessage) *interface{} {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return &v
}

// rawFromAny marshals a Conjure `any` (*interface{}) into a JSONB raw message; nil -> nil (the
// application defaults an empty unit metadata to {}).
func rawFromAny(v *interface{}) json.RawMessage {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(*v)
	if err != nil {
		return nil
	}
	return raw
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
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
