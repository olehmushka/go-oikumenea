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
		Code:       u.Code,
		Name:       u.Name,
		UnitKind:   textPtr(strPtrOrNil(u.UnitKind)),
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
		UnitKind:   textPtr(patch.UnitKind),
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

func (r *Repository) ListUnits(ctx context.Context, level *int, after string, limit int) ([]domain.Unit, error) {
	rows, err := r.q.ListUnits(ctx, tenantsql.ListUnitsParams{
		Level: int2Ptr(level),
		After: textPtr(strPtrOrNil(after)),
		Lim:   int32(limit),
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

// ---------------------------------------------------------------- graphs

func (r *Repository) InsertGraph(ctx context.Context, code, name string, authorityBearing bool) (domain.Graph, error) {
	row, err := r.q.InsertGraph(ctx, tenantsql.InsertGraphParams{
		Code:               code,
		Name:               name,
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

func (r *Repository) GetGraphByCode(ctx context.Context, code string) (domain.Graph, error) {
	row, err := r.q.GetGraphByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Graph{}, domain.ErrGraphNotFound
		}
		return domain.Graph{}, err
	}
	return toGraph(row), nil
}

func (r *Repository) ListGraphs(ctx context.Context) ([]domain.Graph, error) {
	rows, err := r.q.ListGraphs(ctx)
	if err != nil {
		return nil, err
	}
	graphs := make([]domain.Graph, 0, len(rows))
	for _, row := range rows {
		graphs = append(graphs, toGraph(row))
	}
	return graphs, nil
}

func (r *Repository) ClearDefaultGraphs(ctx context.Context) error {
	return r.q.ClearDefaultGraphs(ctx)
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

func (r *Repository) CountActiveGraphs(ctx context.Context) (int, error) {
	n, err := r.q.CountActiveGraphs(ctx)
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
		refs = append(refs, domain.UnitRef{ID: row.ID, Code: row.Code, Name: row.Name, Depth: int(row.Depth), Visibility: domain.Visibility(row.Visibility)})
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
		refs = append(refs, domain.UnitRef{ID: row.ID, Code: row.Code, Name: row.Name, Depth: int(row.Depth), Visibility: domain.Visibility(row.Visibility)})
	}
	return refs, nil
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
		Code:       row.Code,
		Name:       row.Name,
		UnitKind:   row.UnitKind.String, // "" when not valid
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
		Code:               row.Code,
		Name:               row.Name,
		IsDefault:          row.IsDefault,
		IsAuthorityBearing: row.IsAuthorityBearing,
	}
}

// isUniqueViolation reports whether err is a Postgres unique-constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
