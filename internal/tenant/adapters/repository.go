// Package adapters implements the tenant domain ports against infrastructure: the pgx/sqlc
// repository over the oikumenea.tenant_* tables. It depends on the database, never the reverse
// (overview.md). Generated sqlc code lives in the tenantsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/tenant/adapters/tenantsql"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX
// — the pool for reads, or a caller-supplied transaction so a write, its closure recompute, and its
// audit row all commit together (D-Audit / D-ClosureIntegrity).
type Repository struct {
	q *tenantsql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: tenantsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- units

func (r *Repository) InsertUnit(ctx context.Context, u domain.Unit) (domain.Unit, error) {
	metadata := u.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}") // the column is NOT NULL; default empty object
	}
	row, err := r.q.InsertUnit(ctx, tenantsql.InsertUnitParams{
		OrgID:      u.OrgID,
		DomainID:   u.DomainID,
		KindID:     textPtr(u.KindID),
		Code:       textPtr(u.Code),
		Name:       u.Name,
		Level:      int2Ptr(u.Level),
		Visibility: string(u.Visibility),
		Metadata:   metadata,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Unit{}, domain.ErrUnitCodeConflict
		}
		return domain.Unit{}, err
	}
	return toUnit(row), nil
}

func (r *Repository) GetUnit(ctx context.Context, id string) (domain.Unit, error) {
	row, err := r.q.GetUnit(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Unit{}, domain.ErrUnitNotFound
		}
		return domain.Unit{}, err
	}
	return toUnit(row), nil
}

func (r *Repository) UpdateUnit(ctx context.Context, id string, patch domain.UnitPatch) (domain.Unit, error) {
	var visibility *string
	if patch.Visibility != nil {
		v := string(*patch.Visibility)
		visibility = &v
	}
	row, err := r.q.UpdateUnit(ctx, tenantsql.UpdateUnitParams{
		ID:         id,
		Name:       textPtr(patch.Name),
		DomainID:   textPtr(patch.DomainID),
		KindID:     textPtr(patch.KindID),
		Level:      int2Ptr(patch.Level),
		Visibility: textPtr(visibility),
		Metadata:   patch.Metadata, // nil leaves the value unchanged (COALESCE)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Unit{}, domain.ErrUnitNotFound
		}
		return domain.Unit{}, err
	}
	return toUnit(row), nil
}

func (r *Repository) SetUnitState(ctx context.Context, id string, state domain.State) (domain.Unit, error) {
	row, err := r.q.SetUnitState(ctx, tenantsql.SetUnitStateParams{ID: id, State: string(state)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Unit{}, domain.ErrUnitNotFound
		}
		return domain.Unit{}, err
	}
	return toUnit(row), nil
}

func (r *Repository) ListUnits(ctx context.Context, orgID string, domainID, kindID *string, level *int, after string, limit int) ([]domain.Unit, error) {
	rows, err := r.q.ListUnits(ctx, tenantsql.ListUnitsParams{
		OrgID:    orgID,
		DomainID: textPtr(domainID),
		KindID:   textPtr(kindID),
		Level:    int2Ptr(level),
		After:    textPtr(strPtrOrNil(after)),
		Lim:      int32(limit),
	})
	if err != nil {
		return nil, err
	}
	units := make([]domain.Unit, 0, len(rows))
	for _, row := range rows {
		units = append(units, toUnit(row))
	}
	return units, nil
}

// ListChildUnits returns the direct children of parentID within graphID (immediate edges), keyset-paginated.
func (r *Repository) ListChildUnits(ctx context.Context, parentID, graphID, after string, limit int) ([]domain.Unit, error) {
	rows, err := r.q.ListChildUnits(ctx, tenantsql.ListChildUnitsParams{
		GraphID:  graphID,
		ParentID: parentID,
		After:    textPtr(strPtrOrNil(after)),
		Lim:      int32(limit),
	})
	if err != nil {
		return nil, err
	}
	units := make([]domain.Unit, 0, len(rows))
	for _, row := range rows {
		units = append(units, toUnit(row))
	}
	return units, nil
}

// ListRootUnits returns the org's top-level units within graphID (no parent edge), keyset-paginated.
func (r *Repository) ListRootUnits(ctx context.Context, orgID, graphID, after string, limit int) ([]domain.Unit, error) {
	rows, err := r.q.ListRootUnits(ctx, tenantsql.ListRootUnitsParams{
		OrgID:   orgID,
		GraphID: graphID,
		After:   textPtr(strPtrOrNil(after)),
		Lim:     int32(limit),
	})
	if err != nil {
		return nil, err
	}
	units := make([]domain.Unit, 0, len(rows))
	for _, row := range rows {
		units = append(units, toUnit(row))
	}
	return units, nil
}

// ---------------------------------------------------------------- graphs (per-org; orgID nil = global)

func (r *Repository) InsertGraph(ctx context.Context, orgID *string, code, name string, isDefault, authorityBearing bool) (domain.Graph, error) {
	row, err := r.q.InsertGraph(ctx, tenantsql.InsertGraphParams{
		OrgID:              textPtr(orgID),
		Code:               code,
		Name:               name,
		IsDefault:          isDefault,
		IsAuthorityBearing: authorityBearing,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Graph{}, domain.ErrGraphCodeConflict
		}
		return domain.Graph{}, err
	}
	return toGraph(row), nil
}

func (r *Repository) GetGraphByID(ctx context.Context, id string) (domain.Graph, error) {
	row, err := r.q.GetGraphByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Graph{}, domain.ErrGraphNotFound
		}
		return domain.Graph{}, err
	}
	return toGraph(row), nil
}

func (r *Repository) GetGraphForOrgByCode(ctx context.Context, orgID *string, code string) (domain.Graph, error) {
	row, err := r.q.GetGraphForOrgByCode(ctx, tenantsql.GetGraphForOrgByCodeParams{Code: code, OrgID: textPtr(orgID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Graph{}, domain.ErrGraphNotFound
		}
		return domain.Graph{}, err
	}
	return toGraph(row), nil
}

func (r *Repository) ListGraphsForOrg(ctx context.Context, orgID *string) ([]domain.Graph, error) {
	rows, err := r.q.ListGraphsForOrg(ctx, textPtr(orgID))
	if err != nil {
		return nil, err
	}
	graphs := make([]domain.Graph, 0, len(rows))
	for _, row := range rows {
		graphs = append(graphs, toGraph(row))
	}
	return graphs, nil
}

func (r *Repository) ListGraphIDs(ctx context.Context) ([]string, error) {
	return r.q.ListGraphIDs(ctx)
}

func (r *Repository) ClearDefaultGraphsForOrg(ctx context.Context, orgID *string) error {
	return r.q.ClearDefaultGraphsForOrg(ctx, textPtr(orgID))
}

func (r *Repository) UpdateGraph(ctx context.Context, id string, patch domain.GraphPatch) (domain.Graph, error) {
	row, err := r.q.UpdateGraph(ctx, tenantsql.UpdateGraphParams{
		ID:                 id,
		Name:               textPtr(patch.Name),
		IsDefault:          boolPtr(patch.IsDefault),
		IsAuthorityBearing: boolPtr(patch.IsAuthorityBearing),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Graph{}, domain.ErrGraphNotFound
		}
		return domain.Graph{}, err
	}
	return toGraph(row), nil
}

func (r *Repository) SoftDeleteGraph(ctx context.Context, id string) error {
	_, err := r.q.SoftDeleteGraph(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrGraphNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) CountActiveGraphsForOrg(ctx context.Context, orgID *string) (int, error) {
	n, err := r.q.CountActiveGraphsForOrg(ctx, textPtr(orgID))
	return int(n), err
}

func (r *Repository) GraphHasLiveEdges(ctx context.Context, graphID string) (bool, error) {
	return r.q.GraphHasLiveEdges(ctx, graphID)
}

// ---------------------------------------------------------------- edges

func (r *Repository) InsertEdge(ctx context.Context, graphID, parentID, childID, createdBy string) (domain.Edge, error) {
	row, err := r.q.InsertEdge(ctx, tenantsql.InsertEdgeParams{
		GraphID:   graphID,
		ParentID:  parentID,
		ChildID:   childID,
		CreatedBy: textPtr(strPtrOrNil(createdBy)),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Edge{}, domain.ErrEdgeExists
		}
		return domain.Edge{}, err
	}
	return domain.Edge{
		ID:        row.ID,
		ParentID:  row.ParentID,
		ChildID:   row.ChildID,
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *Repository) DeleteEdge(ctx context.Context, graphID, parentID, childID string) (int64, error) {
	return r.q.DeleteEdge(ctx, tenantsql.DeleteEdgeParams{
		GraphID:  graphID,
		ParentID: parentID,
		ChildID:  childID,
	})
}

// ---------------------------------------------------------------- closure

func (r *Repository) ClosureHasPath(ctx context.Context, graphID, ancestorID, descendantID string) (bool, error) {
	return r.q.ClosureHasPath(ctx, tenantsql.ClosureHasPathParams{
		GraphID:      graphID,
		AncestorID:   ancestorID,
		DescendantID: descendantID,
	})
}

// RecomputeClosure rebuilds one graph's full closure from its edges in the caller's transaction:
// truncate the graph's rows, then re-derive them via the recursive query.
func (r *Repository) RecomputeClosure(ctx context.Context, graphID string) error {
	if err := r.q.DeleteClosureForGraph(ctx, graphID); err != nil {
		return err
	}
	return r.q.RebuildClosureForGraph(ctx, graphID)
}

func (r *Repository) VerifyClosure(ctx context.Context, graphID string) (int, int, json.RawMessage, error) {
	row, err := r.q.VerifyClosureForGraph(ctx, graphID)
	if err != nil {
		return 0, 0, nil, err
	}
	var sample json.RawMessage
	if row.Sample != nil {
		raw, err := json.Marshal(row.Sample)
		if err != nil {
			return 0, 0, nil, err
		}
		sample = raw
	}
	return int(row.MissingCount), int(row.ExtraCount), sample, nil
}

func (r *Repository) UpsertClosureStatus(ctx context.Context, graphID string, missing, extra int, inDrift bool, sample json.RawMessage) error {
	return r.q.UpsertClosureStatus(ctx, tenantsql.UpsertClosureStatusParams{
		GraphID:      graphID,
		MissingCount: int32(missing),
		ExtraCount:   int32(extra),
		InDrift:      inDrift,
		Sample:       sample,
	})
}

func (r *Repository) ListAncestors(ctx context.Context, graphID, unitID string) ([]domain.UnitRef, error) {
	rows, err := r.q.ListAncestors(ctx, tenantsql.ListAncestorsParams{GraphID: graphID, UnitID: unitID})
	if err != nil {
		return nil, err
	}
	refs := make([]domain.UnitRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, domain.UnitRef{ID: row.ID, Code: textToPtr(row.Code), Name: row.Name, Depth: int(row.Depth), Visibility: domain.Visibility(row.Visibility)})
	}
	return refs, nil
}

func (r *Repository) ListDescendants(ctx context.Context, graphID, unitID, after string, limit int) ([]domain.UnitRef, error) {
	rows, err := r.q.ListDescendants(ctx, tenantsql.ListDescendantsParams{
		GraphID: graphID,
		UnitID:  unitID,
		After:   textPtr(strPtrOrNil(after)),
		Lim:     int32(limit),
	})
	if err != nil {
		return nil, err
	}
	refs := make([]domain.UnitRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, domain.UnitRef{ID: row.ID, Code: textToPtr(row.Code), Name: row.Name, Depth: int(row.Depth), Visibility: domain.Visibility(row.Visibility)})
	}
	return refs, nil
}

// ---------------------------------------------------------------- code lifecycle (D-UnitCodeLifecycle, M28)

func (r *Repository) SetUnitCode(ctx context.Context, id string, code *string) (domain.Unit, error) {
	row, err := r.q.SetUnitCode(ctx, tenantsql.SetUnitCodeParams{ID: id, Code: textPtr(code)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Unit{}, domain.ErrUnitNotFound
		}
		if isUniqueViolation(err) {
			return domain.Unit{}, domain.ErrUnitCodeConflict
		}
		return domain.Unit{}, err
	}
	return toUnit(row), nil
}

func (r *Repository) CountActiveUnitsByCode(ctx context.Context, code, excludeID string) (int, error) {
	n, err := r.q.CountActiveUnitsByCode(ctx, tenantsql.CountActiveUnitsByCodeParams{
		Code:      pgtype.Text{String: code, Valid: true},
		ExcludeID: excludeID,
	})
	return int(n), err
}

func (r *Repository) InsertUnitCodeEvent(ctx context.Context, e domain.UnitCodeEvent) error {
	return r.q.InsertUnitCodeEvent(ctx, tenantsql.InsertUnitCodeEventParams{
		UnitID:        e.UnitID,
		OldCode:       textPtr(e.OldCode),
		NewCode:       textPtr(e.NewCode),
		Reason:        textPtr(strPtrOrNil(e.Reason)),
		ActorPersonID: textPtr(strPtrOrNil(e.ActorPersonID)),
		RequestID:     e.RequestID,
	})
}

func (r *Repository) ListUnitCodeEvents(ctx context.Context, unitID string) ([]domain.UnitCodeEvent, error) {
	rows, err := r.q.ListUnitCodeEvents(ctx, unitID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.UnitCodeEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.UnitCodeEvent{
			ID:            row.ID,
			UnitID:        row.UnitID,
			OldCode:       textToPtr(row.OldCode),
			NewCode:       textToPtr(row.NewCode),
			Reason:        row.Reason.String,
			ActorPersonID: row.ActorPersonID.String,
			RequestID:     row.RequestID,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- lifecycle

func (r *Repository) InsertLifecycleEvent(ctx context.Context, unitID string, from, to domain.State, reason, actorPersonID, requestID string) error {
	return r.q.InsertLifecycleEvent(ctx, tenantsql.InsertLifecycleEventParams{
		UnitID:        unitID,
		FromState:     string(from),
		ToState:       string(to),
		Reason:        textPtr(strPtrOrNil(reason)),
		ActorPersonID: textPtr(strPtrOrNil(actorPersonID)),
		RequestID:     requestID,
	})
}

// ---------------------------------------------------------------- mapping helpers

func toUnit(row tenantsql.OikumeneaTenantUnit) domain.Unit {
	return domain.Unit{
		ID:         row.ID,
		OrgID:      row.OrgID,
		DomainID:   row.DomainID,
		KindID:     textToPtr(row.KindID),
		Code:       textToPtr(row.Code),
		Name:       row.Name,
		Level:      int2ToPtr(row.Level),
		Visibility: domain.Visibility(row.Visibility),
		State:      domain.State(row.State),
		Metadata:   json.RawMessage(row.Metadata),
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func toGraph(row tenantsql.OikumeneaTenantGraph) domain.Graph {
	return domain.Graph{
		ID:                 row.ID,
		OrgID:              textToPtr(row.OrgID),
		Code:               row.Code,
		Name:               row.Name,
		IsDefault:          row.IsDefault,
		IsAuthorityBearing: row.IsAuthorityBearing,
	}
}

func toDomain(row tenantsql.OikumeneaTenantDomain) domain.Domain {
	return domain.Domain{
		ID:        row.ID,
		Code:      row.Code,
		Name:      row.Name,
		Status:    domain.CatalogStatus(row.Status),
		PdpScoped: row.PdpScoped,
		SortOrder: int4ToPtr(row.SortOrder),
	}
}

func toUnitKind(row tenantsql.OikumeneaTenantUnitKind) domain.UnitKind {
	return domain.UnitKind{
		ID:         row.ID,
		DomainID:   row.DomainID,
		Code:       row.Code,
		Name:       row.Name,
		AttrSchema: json.RawMessage(row.AttrSchema),
		Status:     domain.CatalogStatus(row.Status),
		SortOrder:  int4ToPtr(row.SortOrder),
	}
}

func toOrganization(row tenantsql.OikumeneaTenantOrganization) domain.Organization {
	return domain.Organization{
		ID:         row.ID,
		Code:       row.Code,
		Name:       row.Name,
		DomainID:   row.DomainID,
		Visibility: domain.Visibility(row.Visibility),
		State:      domain.State(row.State),
		Metadata:   json.RawMessage(row.Metadata),
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

// ---------------------------------------------------------------- domains (org-kind catalog, M40)

func (r *Repository) InsertDomain(ctx context.Context, code, name string, sortOrder *int) (domain.Domain, error) {
	row, err := r.q.InsertDomain(ctx, tenantsql.InsertDomainParams{Code: code, Name: name, SortOrder: int4Ptr(sortOrder)})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Domain{}, domain.ErrDomainCodeConflict
		}
		return domain.Domain{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetDomain(ctx context.Context, id string) (domain.Domain, error) {
	row, err := r.q.GetDomain(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Domain{}, domain.ErrDomainNotFound
		}
		return domain.Domain{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) GetDomainByCode(ctx context.Context, code string) (domain.Domain, error) {
	row, err := r.q.GetDomainByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Domain{}, domain.ErrDomainNotFound
		}
		return domain.Domain{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) ListDomains(ctx context.Context) ([]domain.Domain, error) {
	rows, err := r.q.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Domain, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) UpdateDomain(ctx context.Context, id string, patch domain.DomainPatch) (domain.Domain, error) {
	row, err := r.q.UpdateDomain(ctx, tenantsql.UpdateDomainParams{
		ID:        id,
		Name:      textPtr(patch.Name),
		Status:    catalogStatusPtr(patch.Status),
		SortOrder: int4Ptr(patch.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Domain{}, domain.ErrDomainNotFound
		}
		return domain.Domain{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) CountActiveDomainsByCode(ctx context.Context, code, excludeID string) (int, error) {
	n, err := r.q.CountActiveDomainsByCode(ctx, tenantsql.CountActiveDomainsByCodeParams{Code: code, ExcludeID: excludeID})
	return int(n), err
}

// ---------------------------------------------------------------- unit kinds (domain-scoped catalog)

func (r *Repository) InsertUnitKind(ctx context.Context, k domain.UnitKind) (domain.UnitKind, error) {
	row, err := r.q.InsertUnitKind(ctx, tenantsql.InsertUnitKindParams{
		DomainID:   k.DomainID,
		Code:       k.Code,
		Name:       k.Name,
		AttrSchema: jsonOrNil(k.AttrSchema),
		SortOrder:  int4Ptr(k.SortOrder),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.UnitKind{}, domain.ErrUnitKindCodeConflict
		}
		if isFKViolation(err) {
			return domain.UnitKind{}, domain.ErrDomainNotFound
		}
		return domain.UnitKind{}, err
	}
	return toUnitKind(row), nil
}

func (r *Repository) GetUnitKind(ctx context.Context, id string) (domain.UnitKind, error) {
	row, err := r.q.GetUnitKind(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UnitKind{}, domain.ErrUnitKindNotFound
		}
		return domain.UnitKind{}, err
	}
	return toUnitKind(row), nil
}

func (r *Repository) ListUnitKinds(ctx context.Context, domainID string) ([]domain.UnitKind, error) {
	rows, err := r.q.ListUnitKinds(ctx, domainID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.UnitKind, 0, len(rows))
	for _, row := range rows {
		out = append(out, toUnitKind(row))
	}
	return out, nil
}

func (r *Repository) UpdateUnitKind(ctx context.Context, id string, patch domain.UnitKindPatch) (domain.UnitKind, error) {
	row, err := r.q.UpdateUnitKind(ctx, tenantsql.UpdateUnitKindParams{
		ID:         id,
		Name:       textPtr(patch.Name),
		AttrSchema: patch.AttrSchema, // nil leaves unchanged (COALESCE)
		Status:     catalogStatusPtr(patch.Status),
		SortOrder:  int4Ptr(patch.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UnitKind{}, domain.ErrUnitKindNotFound
		}
		return domain.UnitKind{}, err
	}
	return toUnitKind(row), nil
}

func (r *Repository) CountActiveUnitKindsByCode(ctx context.Context, domainID, code, excludeID string) (int, error) {
	n, err := r.q.CountActiveUnitKindsByCode(ctx, tenantsql.CountActiveUnitKindsByCodeParams{
		DomainID:  domainID,
		Code:      code,
		ExcludeID: excludeID,
	})
	return int(n), err
}

// ---------------------------------------------------------------- organizations (the realm)

func (r *Repository) InsertOrganization(ctx context.Context, o domain.Organization) (domain.Organization, error) {
	metadata := o.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	row, err := r.q.InsertOrganization(ctx, tenantsql.InsertOrganizationParams{
		Code:       o.Code,
		Name:       o.Name,
		DomainID:   o.DomainID,
		Visibility: string(o.Visibility),
		Metadata:   metadata,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Organization{}, domain.ErrOrgCodeConflict
		}
		if isFKViolation(err) {
			return domain.Organization{}, domain.ErrDomainNotFound
		}
		return domain.Organization{}, err
	}
	return toOrganization(row), nil
}

func (r *Repository) GetOrganization(ctx context.Context, id string) (domain.Organization, error) {
	row, err := r.q.GetOrganization(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Organization{}, domain.ErrOrgNotFound
		}
		return domain.Organization{}, err
	}
	return toOrganization(row), nil
}

func (r *Repository) UpdateOrganization(ctx context.Context, id string, patch domain.OrgPatch) (domain.Organization, error) {
	var visibility *string
	if patch.Visibility != nil {
		v := string(*patch.Visibility)
		visibility = &v
	}
	row, err := r.q.UpdateOrganization(ctx, tenantsql.UpdateOrganizationParams{
		ID:         id,
		Name:       textPtr(patch.Name),
		DomainID:   textPtr(patch.DomainID),
		Visibility: textPtr(visibility),
		Metadata:   patch.Metadata,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Organization{}, domain.ErrOrgNotFound
		}
		return domain.Organization{}, err
	}
	return toOrganization(row), nil
}

func (r *Repository) SetOrgState(ctx context.Context, id string, state domain.State) (domain.Organization, error) {
	row, err := r.q.SetOrgState(ctx, tenantsql.SetOrgStateParams{ID: id, State: string(state)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Organization{}, domain.ErrOrgNotFound
		}
		return domain.Organization{}, err
	}
	return toOrganization(row), nil
}

func (r *Repository) ListOrganizations(ctx context.Context, domainID *string, after string, limit int) ([]domain.Organization, error) {
	rows, err := r.q.ListOrganizations(ctx, tenantsql.ListOrganizationsParams{
		DomainID: textPtr(domainID),
		After:    textPtr(strPtrOrNil(after)),
		Lim:      int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Organization, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOrganization(row))
	}
	return out, nil
}

func (r *Repository) CountActiveOrgsByCode(ctx context.Context, code, excludeID string) (int, error) {
	n, err := r.q.CountActiveOrgsByCode(ctx, tenantsql.CountActiveOrgsByCodeParams{Code: code, ExcludeID: excludeID})
	return int(n), err
}

func (r *Repository) InsertOrgLifecycleEvent(ctx context.Context, orgID string, from, to domain.State, reason, actorPersonID, requestID string) error {
	return r.q.InsertOrgLifecycleEvent(ctx, tenantsql.InsertOrgLifecycleEventParams{
		OrgID:         orgID,
		FromState:     string(from),
		ToState:       string(to),
		Reason:        textPtr(strPtrOrNil(reason)),
		ActorPersonID: textPtr(strPtrOrNil(actorPersonID)),
		RequestID:     requestID,
	})
}

// isUniqueViolation reports whether err is a Postgres unique-constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isFKViolation reports whether err is a Postgres foreign-key violation (SQLSTATE 23503).
func isFKViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// ---------------------------------------------------------------- unit languages (D-Languages, M18)

func (r *Repository) InsertUnitLanguage(ctx context.Context, l domain.UnitLanguage) error {
	if err := r.q.InsertUnitLanguage(ctx, tenantsql.InsertUnitLanguageParams{
		UnitID:     l.UnitID,
		LanguageID: l.LanguageID,
		IsOfficial: l.IsOfficial,
	}); err != nil {
		return mapUnitLanguageErr(err)
	}
	return nil
}

func (r *Repository) UpdateUnitLanguage(ctx context.Context, l domain.UnitLanguage) error {
	if err := r.q.UpdateUnitLanguage(ctx, tenantsql.UpdateUnitLanguageParams{
		UnitID:     l.UnitID,
		LanguageID: l.LanguageID,
		IsOfficial: l.IsOfficial,
	}); err != nil {
		return mapUnitLanguageErr(err)
	}
	return nil
}

func (r *Repository) GetUnitLanguage(ctx context.Context, unitID, languageID string) (domain.UnitLanguage, error) {
	row, err := r.q.GetUnitLanguage(ctx, tenantsql.GetUnitLanguageParams{UnitID: unitID, LanguageID: languageID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UnitLanguage{}, domain.ErrUnitLanguageNotFound
		}
		return domain.UnitLanguage{}, err
	}
	return domain.UnitLanguage{
		ID:           row.ID,
		UnitID:       row.UnitID,
		LanguageID:   row.LanguageID,
		LanguageName: row.LanguageName,
		IsOfficial:   row.IsOfficial,
	}, nil
}

func (r *Repository) DeleteUnitLanguage(ctx context.Context, unitID, languageID string) error {
	if _, err := r.q.DeleteUnitLanguage(ctx, tenantsql.DeleteUnitLanguageParams{UnitID: unitID, LanguageID: languageID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUnitLanguageNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListUnitLanguages(ctx context.Context, unitID string) ([]domain.UnitLanguage, error) {
	rows, err := r.q.ListUnitLanguages(ctx, unitID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.UnitLanguage, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.UnitLanguage{
			ID:           row.ID,
			UnitID:       row.UnitID,
			LanguageID:   row.LanguageID,
			LanguageName: row.LanguageName,
			IsOfficial:   row.IsOfficial,
		})
	}
	return out, nil
}

// mapUnitLanguageErr maps the unit-language constraint violations: an unresolved languoid FK becomes
// ErrUnknownLanguage; a duplicate active (unit, language) becomes ErrUnitLanguageConflict.
func mapUnitLanguageErr(err error) error {
	switch {
	case isFKViolation(err):
		return domain.ErrUnknownLanguage
	case isUniqueViolation(err):
		return domain.ErrUnitLanguageConflict
	default:
		return err
	}
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func textPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// textToPtr is the inverse: an invalid pgtype.Text (SQL NULL) becomes nil, a valid one a *string.
func textToPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func boolPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func int2Ptr(n *int) pgtype.Int2 {
	if n == nil {
		return pgtype.Int2{}
	}
	return pgtype.Int2{Int16: int16(*n), Valid: true}
}

func int2ToPtr(n pgtype.Int2) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int16)
	return &v
}

func int4Ptr(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*n), Valid: true}
}

func int4ToPtr(n pgtype.Int4) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int32)
	return &v
}

// catalogStatusPtr maps a *domain.CatalogStatus to a nullable text param (nil = unchanged).
func catalogStatusPtr(s *domain.CatalogStatus) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*s), Valid: true}
}

// jsonOrNil passes a JSON value through, or nil when empty (the column is nullable for unit kinds).
func jsonOrNil(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
