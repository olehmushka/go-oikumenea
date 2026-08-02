// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

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
// shadow-visibility gate (F-002, A-lite) is enforced on the unit-result-set reads — ListUnits,
// UnitAncestors, UnitDescendants — as the authoritative app-layer pass (pep.FilterVisibleUnits),
// mirrored at the DB by the tenant_units public-read RLS policy: a `public` unit is broadly
// discoverable, a `shadow` unit appears only when the subject's *.read reaches it. GetUnit stays
// gated by the per-unit Require(read, unitID) (no broadening); membership/order/person/document reads
// remain reach-gated (the A-lite boundary — a public unit is discoverable in listings but its
// roster/detail still needs reach). The bearer token carries the acting subject (interim: token ==
// person RID; see internal/authorization/pep).
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
		OrgID:      req.OrgId,
		DomainID:   derefOr(req.DomainId, ""),
		KindID:     req.KindId,
		Code:       req.Code,
		Name:       req.Name,
		Level:      req.Level,
		Visibility: visibilityOrDefault(req.Visibility),
		Metadata:   rawFromAny(req.Metadata),
	}
	created, err := s.app.CreateUnit(ctx, u)
	if err != nil {
		return tenantapi.Unit{}, s.mapError(ctx, err, errCtx{code: derefOr(req.Code, ""), orgID: req.OrgId})
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
		DomainID: req.DomainId,
		KindID:   req.KindId,
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

// SetUnitCode implements PUT /units/{unitId}/code — the audited set/correct/clear of a unit's code
// (D-UnitCodeLifecycle, M28). An omitted code clears it. Gated by unit.recode at the path unit.
func (s Service) SetUnitCode(ctx context.Context, token bearertoken.Token, unitID string, req tenantapi.SetUnitCodeRequest) (tenantapi.Unit, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitRecode), unitID); err != nil {
		return tenantapi.Unit{}, err
	}
	updated, err := s.app.SetUnitCode(ctx, unitID, req.Code, derefOr(req.Reason, ""))
	if err != nil {
		return tenantapi.Unit{}, s.mapError(ctx, err, errCtx{unitID: unitID, code: derefOr(req.Code, "")})
	}
	return s.unitToAPI(ctx, updated)
}

// ListUnitCodeEvents implements GET /units/{unitId}/code-events — a unit's code-change history.
func (s Service) ListUnitCodeEvents(ctx context.Context, token bearertoken.Token, unitID string) (tenantapi.UnitCodeEventList, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitRead), unitID); err != nil {
		return tenantapi.UnitCodeEventList{}, err
	}
	events, err := s.app.ListUnitCodeEvents(ctx, unitID)
	if err != nil {
		return tenantapi.UnitCodeEventList{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	out := make([]tenantapi.UnitCodeEvent, 0, len(events))
	for _, e := range events {
		out = append(out, tenantapi.UnitCodeEvent{
			Id:        e.ID,
			UnitId:    e.UnitID,
			OldCode:   e.OldCode,
			NewCode:   e.NewCode,
			Reason:    strPtrOrNil(e.Reason),
			CreatedAt: datetime.DateTime(e.CreatedAt),
		})
	}
	return tenantapi.UnitCodeEventList{Events: out}, nil
}

func (s Service) ListUnits(ctx context.Context, token bearertoken.Token, org string, domainID *string, unitKind *string, level *int, levelMin *int, levelMax *int, visibility *string, state *string, pdpScoped *bool, graph *string, parent *string, rootsOnly *bool, pageSize *int, pageToken *string) (tenantapi.UnitPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermUnitRead)); err != nil {
		return tenantapi.UnitPage{}, err
	}
	// One filter struct for the whole vocabulary (M56 / D-ObjectFacets); the application layer
	// validates it, so an ill-formed facet value is a 400 and never reaches SQL.
	filter := domain.UnitFilter{
		OrgID:      org,
		DomainID:   domainID,
		KindID:     unitKind,
		Level:      level,
		LevelMin:   levelMin,
		LevelMax:   levelMax,
		Visibility: visibility,
		State:      state,
		PDPScoped:  pdpScoped,
	}
	page, err := s.app.ListUnits(ctx, filter, derefOr(graph, ""), parent, derefOr(rootsOnly, false), derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return tenantapi.UnitPage{}, s.mapError(ctx, err, errCtx{orgID: org})
	}
	visible, err := gateUnits(ctx, s.pep, page.Units, func(u domain.Unit) string { return u.ID }, func(u domain.Unit) bool { return u.Visibility == domain.VisibilityShadow })
	if err != nil {
		return tenantapi.UnitPage{}, s.mapError(ctx, err, errCtx{})
	}
	units, err := s.unitsToAPI(ctx, visible)
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

// ---------------------------------------------------------------- unit languages (D-Languages, M18)

// ListUnitLanguages implements GET /units/{unitId}/languages.
func (s Service) ListUnitLanguages(ctx context.Context, token bearertoken.Token, unitID string) ([]tenantapi.UnitLanguage, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitRead), unitID); err != nil {
		return nil, err
	}
	ls, err := s.app.ListUnitLanguages(ctx, unitID)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return s.unitLanguagesToAPI(ctx, ls)
}

// UpsertUnitLanguage implements PUT /units/{unitId}/languages.
func (s Service) UpsertUnitLanguage(ctx context.Context, token bearertoken.Token, unitID string, req tenantapi.UpsertUnitLanguageRequest) (tenantapi.UnitLanguage, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitUpdate), unitID); err != nil {
		return tenantapi.UnitLanguage{}, err
	}
	saved, err := s.app.UpsertUnitLanguage(ctx, domain.UnitLanguage{
		UnitID:     unitID,
		LanguageID: req.LanguageId,
		IsOfficial: derefOr(req.IsOfficial, true),
	})
	if err != nil {
		return tenantapi.UnitLanguage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	out, err := s.unitLanguagesToAPI(ctx, []domain.UnitLanguage{saved})
	if err != nil {
		return tenantapi.UnitLanguage{}, err
	}
	return out[0], nil
}

// DeleteUnitLanguage implements DELETE /units/{unitId}/languages/{languageId}.
func (s Service) DeleteUnitLanguage(ctx context.Context, token bearertoken.Token, unitID, languageID string) error {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitUpdate), unitID); err != nil {
		return err
	}
	if err := s.app.DeleteUnitLanguage(ctx, unitID, languageID); err != nil {
		return s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return nil
}

// unitLanguagesToAPI maps the domain rows to the API shape, assembling each languoid's locale->text
// name map (D-i18n) from the default-locale name + the localization store (entity type "languoid").
func (s Service) unitLanguagesToAPI(ctx context.Context, ls []domain.UnitLanguage) ([]tenantapi.UnitLanguage, error) {
	defaults := make(map[string]string, len(ls))
	for _, l := range ls {
		defaults[l.LanguageID] = l.LanguageName
	}
	names, err := s.loc.NamesByID(ctx, "languoid", defaults)
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.UnitLanguage, 0, len(ls))
	for _, l := range ls {
		out = append(out, tenantapi.UnitLanguage{
			Id:         l.ID,
			UnitId:     l.UnitID,
			LanguageId: l.LanguageID,
			Name:       names[l.LanguageID],
			IsOfficial: l.IsOfficial,
		})
	}
	return out, nil
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
	refs, err = gateUnits(ctx, s.pep, refs, func(r domain.UnitRef) string { return r.ID }, func(r domain.UnitRef) bool { return r.Visibility == domain.VisibilityShadow })
	if err != nil {
		return tenantapi.UnitRefList{}, s.mapError(ctx, err, errCtx{unitID: unitID})
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
	page.Refs, err = gateUnits(ctx, s.pep, page.Refs, func(r domain.UnitRef) string { return r.ID }, func(r domain.UnitRef) bool { return r.Visibility == domain.VisibilityShadow })
	if err != nil {
		return tenantapi.UnitRefPage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	out, err := s.refsToAPI(ctx, page.Refs)
	if err != nil {
		return tenantapi.UnitRefPage{}, s.mapError(ctx, err, errCtx{unitID: unitID})
	}
	return tenantapi.UnitRefPage{Units: out, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

// gateUnits applies the shadow-visibility gate (F-002) to a slice of unit-like items as the
// authoritative app-layer pass: it drops any `shadow` unit the request subject's *.read does not
// reach, while `public` and reachable units pass, preserving order. `id`/`shadow` extract the unit
// RID and its shadow flag from an item, so it serves both domain.Unit (ListUnits) and domain.UnitRef
// (ancestors/descendants). The gate logic itself is owned by authorization (pep.FilterVisibleUnits);
// this only adapts the result back onto the typed items the caller returns.
func gateUnits[T any](ctx context.Context, enf *pep.Enforcer, items []T, id func(T) string, shadow func(T) bool) ([]T, error) {
	if len(items) == 0 {
		return items, nil
	}
	ids := make([]string, len(items))
	shadowMap := make(map[string]bool, len(items))
	for i, it := range items {
		uid := id(it)
		ids[i] = uid
		shadowMap[uid] = shadow(it)
	}
	visible, err := enf.FilterVisibleUnits(ctx, ids, shadowMap)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(visible))
	for _, u := range visible {
		allowed[u] = struct{}{}
	}
	out := make([]T, 0, len(items))
	for _, it := range items {
		if _, ok := allowed[id(it)]; ok {
			out = append(out, it)
		}
	}
	return out, nil
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

func (s Service) ListGraphs(ctx context.Context, token bearertoken.Token, org *string) (tenantapi.GraphList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermGraphRead)); err != nil {
		return tenantapi.GraphList{}, err
	}
	graphs, err := s.app.ListGraphs(ctx, org)
	if err != nil {
		return tenantapi.GraphList{}, s.mapError(ctx, err, errCtx{orgID: derefOr(org, "")})
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
	created, err := s.app.AddGraph(ctx, req.OrgId, req.Code, req.Name, derefOr(req.IsAuthorityBearing, true))
	if err != nil {
		return tenantapi.Graph{}, s.mapError(ctx, err, errCtx{code: req.Code, orgID: derefOr(req.OrgId, "")})
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

// ---------------------------------------------------------------- domains (M40)

func (s Service) ListDomains(ctx context.Context, token bearertoken.Token) (tenantapi.DomainList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDomainRead)); err != nil {
		return tenantapi.DomainList{}, err
	}
	ds, err := s.app.ListDomains(ctx)
	if err != nil {
		return tenantapi.DomainList{}, s.mapError(ctx, err, errCtx{})
	}
	out, err := s.domainsToAPI(ctx, ds)
	if err != nil {
		return tenantapi.DomainList{}, s.mapError(ctx, err, errCtx{})
	}
	return tenantapi.DomainList{Domains: out}, nil
}

func (s Service) CreateDomain(ctx context.Context, token bearertoken.Token, req tenantapi.CreateDomainRequest) (tenantapi.Domain, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermDomainManage), ""); err != nil {
		return tenantapi.Domain{}, err
	}
	created, err := s.app.CreateDomain(ctx, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return tenantapi.Domain{}, s.mapError(ctx, err, errCtx{code: req.Code})
	}
	return s.domainToAPI(ctx, created)
}

func (s Service) UpdateDomain(ctx context.Context, token bearertoken.Token, domainID string, req tenantapi.UpdateDomainRequest) (tenantapi.Domain, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermDomainManage), ""); err != nil {
		return tenantapi.Domain{}, err
	}
	updated, err := s.app.UpdateDomain(ctx, domainID, domain.DomainPatch{
		Name:      req.Name,
		Status:    catalogStatusPtr(req.Status),
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return tenantapi.Domain{}, s.mapError(ctx, err, errCtx{domainID: domainID})
	}
	return s.domainToAPI(ctx, updated)
}

// ---------------------------------------------------------------- unit kinds (M40)

func (s Service) ListUnitKinds(ctx context.Context, token bearertoken.Token, domainID string) (tenantapi.UnitKindList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermUnitKindRead)); err != nil {
		return tenantapi.UnitKindList{}, err
	}
	ks, err := s.app.ListUnitKinds(ctx, domainID)
	if err != nil {
		return tenantapi.UnitKindList{}, s.mapError(ctx, err, errCtx{domainID: domainID})
	}
	out, err := s.unitKindsToAPI(ctx, ks)
	if err != nil {
		return tenantapi.UnitKindList{}, s.mapError(ctx, err, errCtx{domainID: domainID})
	}
	return tenantapi.UnitKindList{UnitKinds: out}, nil
}

func (s Service) CreateUnitKind(ctx context.Context, token bearertoken.Token, req tenantapi.CreateUnitKindRequest) (tenantapi.UnitKind, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitKindManage), ""); err != nil {
		return tenantapi.UnitKind{}, err
	}
	created, err := s.app.CreateUnitKind(ctx, domain.UnitKind{
		DomainID:   req.DomainId,
		Code:       req.Code,
		Name:       req.Name,
		AttrSchema: rawFromAny(req.AttrSchema),
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		return tenantapi.UnitKind{}, s.mapError(ctx, err, errCtx{domainID: req.DomainId, code: req.Code})
	}
	return s.unitKindToAPI(ctx, created)
}

func (s Service) UpdateUnitKind(ctx context.Context, token bearertoken.Token, unitKindID string, req tenantapi.UpdateUnitKindRequest) (tenantapi.UnitKind, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermUnitKindManage), ""); err != nil {
		return tenantapi.UnitKind{}, err
	}
	updated, err := s.app.UpdateUnitKind(ctx, unitKindID, domain.UnitKindPatch{
		Name:       req.Name,
		AttrSchema: rawFromAny(req.AttrSchema),
		Status:     catalogStatusPtr(req.Status),
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		return tenantapi.UnitKind{}, s.mapError(ctx, err, errCtx{unitKindID: unitKindID})
	}
	return s.unitKindToAPI(ctx, updated)
}

// ---------------------------------------------------------------- organizations (M40)

func (s Service) ListOrganizations(ctx context.Context, token bearertoken.Token, domainID *string, visibility *string, state *string, pageSize *int, pageToken *string) (tenantapi.OrganizationPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrganizationRead)); err != nil {
		return tenantapi.OrganizationPage{}, err
	}
	page, err := s.app.ListOrganizations(ctx, orgFilterFrom(domainID, visibility, state), derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return tenantapi.OrganizationPage{}, s.mapError(ctx, err, errCtx{})
	}
	visible, err := gateUnits(ctx, s.pep, page.Orgs, func(o domain.Organization) string { return o.ID }, func(o domain.Organization) bool { return o.Visibility == domain.VisibilityShadow })
	if err != nil {
		return tenantapi.OrganizationPage{}, s.mapError(ctx, err, errCtx{})
	}
	out, err := s.organizationsToAPI(ctx, visible)
	if err != nil {
		return tenantapi.OrganizationPage{}, s.mapError(ctx, err, errCtx{})
	}
	return tenantapi.OrganizationPage{Organizations: out, NextPageToken: tokenPtr(page.NextPageToken)}, nil
}

func (s Service) CreateOrganization(ctx context.Context, token bearertoken.Token, req tenantapi.CreateOrganizationRequest) (tenantapi.Organization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrganizationCreate)); err != nil {
		return tenantapi.Organization{}, err
	}
	created, err := s.app.CreateOrganization(ctx, domain.Organization{
		Code:       req.Code,
		Name:       req.Name,
		DomainID:   req.DomainId,
		Visibility: visibilityOrDefault(req.Visibility),
		Metadata:   rawFromAny(req.Metadata),
	})
	if err != nil {
		return tenantapi.Organization{}, s.mapError(ctx, err, errCtx{code: req.Code, domainID: req.DomainId})
	}
	return s.organizationToAPI(ctx, created)
}

// GetOrganization reads one organization by RID, shadow-gated — which it was NOT before M58 ticket 4
// despite the contract saying so. `listOrganizations` trims shadow organizations a caller cannot
// reach; this point read applied `organization.read` and handed the row over, so anyone holding that
// code could read any shadow organization by RID. The list hid it and the point read did not.
//
// The gate is `gateUnits`, the same helper the list uses, deliberately rather than an inlined
// visibility check: the rule then has ONE implementation, so when organization reachability is fixed
// (facets.md open seam — an organization RID can never appear in a role assignment's reach today)
// both surfaces move together instead of one being remembered and the other not.
//
// A gated-out organization is `OrganizationNotFound`, NOT a permission error. `shadow` exists to hide
// EXISTENCE (F-002 / D-VisibilityScope), and a 403 would confirm that the RID names a real
// organization — which is exactly what the list refuses to say by omitting the row.
func (s Service) GetOrganization(ctx context.Context, token bearertoken.Token, orgID string) (tenantapi.Organization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrganizationRead)); err != nil {
		return tenantapi.Organization{}, err
	}
	o, err := s.app.GetOrganization(ctx, orgID)
	if err != nil {
		return tenantapi.Organization{}, s.mapError(ctx, err, errCtx{orgID: orgID})
	}
	visible, err := gateUnits(ctx, s.pep, []domain.Organization{o},
		func(o domain.Organization) string { return o.ID },
		func(o domain.Organization) bool { return o.Visibility == domain.VisibilityShadow })
	if err != nil {
		return tenantapi.Organization{}, s.mapError(ctx, err, errCtx{orgID: orgID})
	}
	if len(visible) == 0 {
		return tenantapi.Organization{}, s.mapError(ctx, domain.ErrOrgNotFound, errCtx{orgID: orgID})
	}
	return s.organizationToAPI(ctx, visible[0])
}

func (s Service) UpdateOrganization(ctx context.Context, token bearertoken.Token, orgID string, req tenantapi.UpdateOrganizationRequest) (tenantapi.Organization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrganizationUpdate)); err != nil {
		return tenantapi.Organization{}, err
	}
	patch := domain.OrgPatch{
		Name:     req.Name,
		DomainID: req.DomainId,
		Metadata: rawFromAny(req.Metadata),
	}
	if req.Visibility != nil {
		v := fromAPIVisibility(*req.Visibility)
		patch.Visibility = &v
	}
	updated, err := s.app.UpdateOrganization(ctx, orgID, patch)
	if err != nil {
		return tenantapi.Organization{}, s.mapError(ctx, err, errCtx{orgID: orgID})
	}
	return s.organizationToAPI(ctx, updated)
}

func (s Service) TransitionOrganization(ctx context.Context, token bearertoken.Token, orgID string, req tenantapi.TransitionRequest) (tenantapi.Organization, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrganizationLifecycle)); err != nil {
		return tenantapi.Organization{}, err
	}
	updated, err := s.app.TransitionOrganization(ctx, orgID, fromAPIState(req.ToState), derefOr(req.Reason, ""))
	if err != nil {
		return tenantapi.Organization{}, s.mapError(ctx, err, errCtx{orgID: orgID})
	}
	return s.organizationToAPI(ctx, updated)
}

// ListOrganizationGraphs is the path-scoped alias of GET /graphs?org=.
func (s Service) ListOrganizationGraphs(ctx context.Context, token bearertoken.Token, orgID string) (tenantapi.GraphList, error) {
	return s.ListGraphs(ctx, token, &orgID)
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

func (s Service) domainToAPI(ctx context.Context, d domain.Domain) (tenantapi.Domain, error) {
	names, err := s.loc.NamesByID(ctx, "domain", map[string]string{d.ID: d.Name})
	if err != nil {
		return tenantapi.Domain{}, err
	}
	return toAPIDomain(d, names[d.ID]), nil
}

func (s Service) domainsToAPI(ctx context.Context, ds []domain.Domain) ([]tenantapi.Domain, error) {
	defaults := make(map[string]string, len(ds))
	for _, d := range ds {
		defaults[d.ID] = d.Name
	}
	names, err := s.loc.NamesByID(ctx, "domain", defaults)
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.Domain, 0, len(ds))
	for _, d := range ds {
		out = append(out, toAPIDomain(d, names[d.ID]))
	}
	return out, nil
}

func (s Service) unitKindToAPI(ctx context.Context, k domain.UnitKind) (tenantapi.UnitKind, error) {
	names, err := s.loc.NamesByID(ctx, "unit_kind", map[string]string{k.ID: k.Name})
	if err != nil {
		return tenantapi.UnitKind{}, err
	}
	return toAPIUnitKind(k, names[k.ID]), nil
}

func (s Service) unitKindsToAPI(ctx context.Context, ks []domain.UnitKind) ([]tenantapi.UnitKind, error) {
	defaults := make(map[string]string, len(ks))
	for _, k := range ks {
		defaults[k.ID] = k.Name
	}
	names, err := s.loc.NamesByID(ctx, "unit_kind", defaults)
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.UnitKind, 0, len(ks))
	for _, k := range ks {
		out = append(out, toAPIUnitKind(k, names[k.ID]))
	}
	return out, nil
}

func (s Service) organizationToAPI(ctx context.Context, o domain.Organization) (tenantapi.Organization, error) {
	names, err := s.loc.NamesByID(ctx, "organization", map[string]string{o.ID: o.Name})
	if err != nil {
		return tenantapi.Organization{}, err
	}
	return toAPIOrganization(o, names[o.ID]), nil
}

func (s Service) organizationsToAPI(ctx context.Context, os []domain.Organization) ([]tenantapi.Organization, error) {
	defaults := make(map[string]string, len(os))
	for _, o := range os {
		defaults[o.ID] = o.Name
	}
	names, err := s.loc.NamesByID(ctx, "organization", defaults)
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.Organization, 0, len(os))
	for _, o := range os {
		out = append(out, toAPIOrganization(o, names[o.ID]))
	}
	return out, nil
}

func toAPIDomain(d domain.Domain, name map[string]string) tenantapi.Domain {
	return tenantapi.Domain{
		Id:        d.ID,
		Code:      d.Code,
		Name:      name,
		Status:    string(d.Status),
		SortOrder: d.SortOrder,
	}
}

func toAPIUnitKind(k domain.UnitKind, name map[string]string) tenantapi.UnitKind {
	return tenantapi.UnitKind{
		Id:         k.ID,
		DomainId:   k.DomainID,
		Code:       k.Code,
		Name:       name,
		AttrSchema: anyFromRaw(k.AttrSchema),
		Status:     string(k.Status),
		SortOrder:  k.SortOrder,
	}
}

func toAPIOrganization(o domain.Organization, name map[string]string) tenantapi.Organization {
	return tenantapi.Organization{
		Id:         o.ID,
		Code:       o.Code,
		Name:       name,
		DomainId:   o.DomainID,
		Visibility: toAPIVisibility(o.Visibility),
		State:      toAPIState(o.State),
		Metadata:   anyFromRaw(o.Metadata),
		CreatedAt:  datetime.DateTime(o.CreatedAt),
		UpdatedAt:  datetime.DateTime(o.UpdatedAt),
	}
}

// catalogStatusPtr maps an API status string pointer to a *domain.CatalogStatus (nil = unchanged).
func catalogStatusPtr(s *string) *domain.CatalogStatus {
	if s == nil {
		return nil
	}
	cs := domain.CatalogStatus(*s)
	return &cs
}

func toAPIUnit(u domain.Unit, name map[string]string) tenantapi.Unit {
	return tenantapi.Unit{
		Id:         u.ID,
		OrgId:      u.OrgID,
		DomainId:   u.DomainID,
		KindId:     u.KindID,
		Code:       u.Code,
		Name:       name,
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
		OrgId:              g.OrgID,
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
	unitID     string
	orgID      string
	domainID   string
	unitKindID string
	code       string
	graph      string
	parentID   string
	childID    string
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
	case errors.Is(err, domain.ErrInvalidOrg):
		return tenantapi.NewOrganizationInvalid(err.Error())
	case errors.Is(err, domain.ErrGraphNotFound):
		return tenantapi.NewGraphNotFound(c.graph)
	case errors.Is(err, domain.ErrGraphCodeConflict):
		return tenantapi.NewGraphCodeConflict(c.code)
	case errors.Is(err, domain.ErrGraphInUse):
		return tenantapi.NewGraphInUse(c.graph)
	case errors.Is(err, domain.ErrGraphProtected):
		return tenantapi.NewGraphProtected(err.Error())
	case errors.Is(err, domain.ErrDomainNotFound):
		return tenantapi.NewDomainNotFound(c.domainID)
	case errors.Is(err, domain.ErrDomainCodeConflict):
		return tenantapi.NewDomainCodeConflict(c.code)
	case errors.Is(err, domain.ErrUnitKindNotFound):
		return tenantapi.NewUnitKindNotFound(c.unitKindID)
	case errors.Is(err, domain.ErrUnitKindCodeConflict):
		return tenantapi.NewUnitKindCodeConflict(c.domainID, c.code)
	case errors.Is(err, domain.ErrOrgNotFound):
		return tenantapi.NewOrganizationNotFound(c.orgID)
	case errors.Is(err, domain.ErrOrgCodeConflict):
		return tenantapi.NewOrganizationCodeConflict(c.code)
	case errors.Is(err, domain.ErrUnknownLanguage):
		return tenantapi.NewUnitInvalid("language does not exist")
	case errors.Is(err, domain.ErrUnitLanguageConflict):
		return tenantapi.NewUnitInvalid("the unit already has this language")
	case errors.Is(err, domain.ErrUnitLanguageNotFound):
		return tenantapi.NewUnitNotFound(c.unitID)
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
