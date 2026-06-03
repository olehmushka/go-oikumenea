// Package application holds the tenant module's application service — the orchestrator the transport
// layer calls to read/mutate the unit graph, and that maintains the per-graph closure and records
// audit rows in the same transaction as each write (D-Audit / D-ClosureIntegrity). It depends on
// the domain port, the platform DB surface, and the audit service; it never imports the adapters
// package directly (the repository factory is injected by module.go).
package application

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// auditSubsystem labels the interim system actor for tenant's admin writes. Until authorization
// (M7) + identity-federation (M8) resolve the acting person, these writes are recorded as a
// `system` action under this subsystem (the no-unaudited-mutation ground rule still holds). M7/M8
// replace this with the resolved person actor.
const auditSubsystem = "tenant-admin"

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an audited write (D-Audit). Injected by module.go so the application
// layer never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the tenant application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
}

// NewService wires the service with the pool, the repository factory, and the audit service every
// write records into.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// UnitPage is a page of units plus the opaque next-page token (empty when exhausted).
type UnitPage struct {
	Units         []domain.Unit
	NextPageToken string
}

// UnitRefPage is a page of unit references plus the opaque next-page token.
type UnitRefPage struct {
	Refs          []domain.UnitRef
	NextPageToken string
}

// ---------------------------------------------------------------- units

// CreateUnit validates and creates a unit, then records the action. Creating a root unit (no
// parent) is the first post-bootstrap action; edges are added separately.
func (s *Service) CreateUnit(ctx context.Context, u domain.Unit) (domain.Unit, error) {
	if u.Visibility == "" {
		u.Visibility = domain.VisibilityPublic
	}
	if u.State == "" {
		u.State = domain.StateActive
	}
	if err := u.Validate(); err != nil {
		return domain.Unit{}, err
	}
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertUnit(ctx, u)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "unit.create", "unit", created.ID, created.ID, created)
	})
	return out, err
}

// GetUnit reads one unit, returning domain.ErrUnitNotFound when absent or soft-deleted.
func (s *Service) GetUnit(ctx context.Context, id string) (domain.Unit, error) {
	return s.newRepo(s.pool).GetUnit(ctx, id)
}

// UpdateUnit applies a partial change (name/kind/level/metadata/visibility) and records the action.
// `code` is immutable by convention and not patchable.
func (s *Service) UpdateUnit(ctx context.Context, id string, patch domain.UnitPatch) (domain.Unit, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.Unit{}, domain.ErrInvalidUnit
	}
	if patch.Visibility != nil && *patch.Visibility != domain.VisibilityPublic && *patch.Visibility != domain.VisibilityShadow {
		return domain.Unit{}, domain.ErrInvalidUnit
	}
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateUnit(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "unit.update", "unit", id, id, updated)
	})
	return out, err
}

// ListUnits returns a keyset-paginated page of units (by time-ordered RID), optionally filtered by
// level.
func (s *Service) ListUnits(ctx context.Context, level *int, pageSize int, pageToken string) (UnitPage, error) {
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return UnitPage{}, err
	}
	units, err := s.newRepo(s.pool).ListUnits(ctx, level, after, size+1)
	if err != nil {
		return UnitPage{}, err
	}
	if len(units) > size {
		last := units[size-1]
		return UnitPage{Units: units[:size], NextPageToken: encodeCursor(last.ID)}, nil
	}
	return UnitPage{Units: units}, nil
}

// TransitionUnit moves a unit to a new lifecycle state, appends the append-only lifecycle event,
// and records the action — all in one transaction. An illegal transition is rejected before any
// write (domain.ErrInvalidTransition).
func (s *Service) TransitionUnit(ctx context.Context, id string, to domain.State, reason string) (domain.Unit, error) {
	if !domain.ValidState(to) {
		return domain.Unit{}, domain.ErrInvalidTransition
	}
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		current, err := repo.GetUnit(ctx, id)
		if err != nil {
			return err
		}
		if !current.State.CanTransitionTo(to) {
			return domain.ErrInvalidTransition
		}
		updated, err := repo.SetUnitState(ctx, id, to)
		if err != nil {
			return err
		}
		if err := repo.InsertLifecycleEvent(ctx, id, current.State, to, reason, "", requestID(ctx)); err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "unit.transition", "unit", id, id, map[string]string{
			"from": string(current.State), "to": string(to), "reason": reason,
		})
	})
	return out, err
}

// ---------------------------------------------------------------- edges

// AddEdge attaches childID as a child of parentID within a graph (default command), guarding
// against cycles, then recomputes the graph's closure and records the action — all in one
// transaction. graphCode "" resolves to the command graph.
func (s *Service) AddEdge(ctx context.Context, childID, parentID, graphCode string) (domain.Edge, error) {
	graphCode = defaultGraph(graphCode)
	if parentID == childID {
		return domain.Edge{}, domain.ErrUnitCycle // a self-loop is the degenerate cycle
	}
	var out domain.Edge
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		g, err := repo.GetGraphByCode(ctx, graphCode)
		if err != nil {
			return err
		}
		if _, err := repo.GetUnit(ctx, parentID); err != nil {
			return err
		}
		if _, err := repo.GetUnit(ctx, childID); err != nil {
			return err
		}
		// A new parent->child edge closes a cycle iff the child already reaches the parent in g.
		cyclic, err := repo.ClosureHasPath(ctx, g.ID, childID, parentID)
		if err != nil {
			return err
		}
		if cyclic {
			return domain.ErrUnitCycle
		}
		edge, err := repo.InsertEdge(ctx, g.ID, parentID, childID, "")
		if err != nil {
			return err
		}
		if err := repo.RecomputeClosure(ctx, g.ID); err != nil {
			return err
		}
		edge.Graph = g.Code
		out = edge
		return s.record(ctx, tx, "unit.edge.add", "unit", childID, childID, map[string]string{
			"graph": g.Code, "parentId": parentID, "childId": childID,
		})
	})
	return out, err
}

// RemoveEdge detaches childID from parentID within a graph (default command), recomputes the
// graph's closure, and records the action. Detaching an absent edge is a no-op (idempotent).
func (s *Service) RemoveEdge(ctx context.Context, childID, parentID, graphCode string) error {
	graphCode = defaultGraph(graphCode)
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		g, err := repo.GetGraphByCode(ctx, graphCode)
		if err != nil {
			return err
		}
		if _, err := repo.DeleteEdge(ctx, g.ID, parentID, childID); err != nil {
			return err
		}
		if err := repo.RecomputeClosure(ctx, g.ID); err != nil {
			return err
		}
		return s.record(ctx, tx, "unit.edge.remove", "unit", childID, childID, map[string]string{
			"graph": g.Code, "parentId": parentID, "childId": childID,
		})
	})
}

// Ancestors returns the unit's ancestors in graph graphCode (default command), nearest first.
func (s *Service) Ancestors(ctx context.Context, unitID, graphCode string) ([]domain.UnitRef, error) {
	repo := s.newRepo(s.pool)
	g, err := repo.GetGraphByCode(ctx, defaultGraph(graphCode))
	if err != nil {
		return nil, err
	}
	if _, err := repo.GetUnit(ctx, unitID); err != nil {
		return nil, err
	}
	return repo.ListAncestors(ctx, g.ID, unitID)
}

// Descendants returns a keyset-paginated page of the unit's subtree in graph graphCode (default
// command).
func (s *Service) Descendants(ctx context.Context, unitID, graphCode string, pageSize int, pageToken string) (UnitRefPage, error) {
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return UnitRefPage{}, err
	}
	repo := s.newRepo(s.pool)
	g, err := repo.GetGraphByCode(ctx, defaultGraph(graphCode))
	if err != nil {
		return UnitRefPage{}, err
	}
	if _, err := repo.GetUnit(ctx, unitID); err != nil {
		return UnitRefPage{}, err
	}
	refs, err := repo.ListDescendants(ctx, g.ID, unitID, after, size+1)
	if err != nil {
		return UnitRefPage{}, err
	}
	if len(refs) > size {
		last := refs[size-1]
		return UnitRefPage{Refs: refs[:size], NextPageToken: encodeCursor(last.ID)}, nil
	}
	return UnitRefPage{Refs: refs}, nil
}

// ---------------------------------------------------------------- closure integrity

// VerifyClosure diffs the stored closure vs. the edges per graph and upserts the per-graph drift
// status the closure-drift health reporter reads (default: all graphs). One transaction.
func (s *Service) VerifyClosure(ctx context.Context, graphCode *string) ([]domain.ClosureReport, error) {
	graphs, err := s.resolveGraphs(ctx, graphCode)
	if err != nil {
		return nil, err
	}
	var reports []domain.ClosureReport
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		reports = reports[:0]
		for _, g := range graphs {
			missing, extra, sample, err := repo.VerifyClosure(ctx, g.ID)
			if err != nil {
				return err
			}
			inDrift := missing > 0 || extra > 0
			if err := repo.UpsertClosureStatus(ctx, g.ID, missing, extra, inDrift, sample); err != nil {
				return err
			}
			reports = append(reports, domain.ClosureReport{
				Graph: g.Code, MissingCount: missing, ExtraCount: extra, InDrift: inDrift, Sample: sample,
			})
		}
		return s.record(ctx, tx, "closure.verify", "graph", singleTargetID(graphs), "", reports)
	})
	return reports, err
}

// RebuildClosure truncates + recomputes the closure, one transaction per graph (default: all
// graphs); each is an audited write. After a rebuild the graph is consistent by construction, so
// its drift status is reset to zero.
func (s *Service) RebuildClosure(ctx context.Context, graphCode *string) ([]domain.ClosureReport, error) {
	graphs, err := s.resolveGraphs(ctx, graphCode)
	if err != nil {
		return nil, err
	}
	reports := make([]domain.ClosureReport, 0, len(graphs))
	for _, g := range graphs {
		if err := s.inTx(ctx, func(tx pgx.Tx) error {
			repo := s.newRepo(tx)
			if err := repo.RecomputeClosure(ctx, g.ID); err != nil {
				return err
			}
			if err := repo.UpsertClosureStatus(ctx, g.ID, 0, 0, false, nil); err != nil {
				return err
			}
			return s.record(ctx, tx, "closure.rebuild", "graph", g.ID, "", map[string]string{"graph": g.Code})
		}); err != nil {
			return nil, err
		}
		reports = append(reports, domain.ClosureReport{Graph: g.Code})
	}
	return reports, nil
}

// ---------------------------------------------------------------- graphs

// ListGraphs returns the graph registry in display order.
func (s *Service) ListGraphs(ctx context.Context) ([]domain.Graph, error) {
	return s.newRepo(s.pool).ListGraphs(ctx)
}

// AddGraph validates and adds a graph (instance-admin) and records the action. New graphs are never
// the default (promote via UpdateGraph).
func (s *Service) AddGraph(ctx context.Context, code, name string, authorityBearing bool) (domain.Graph, error) {
	g := domain.Graph{Code: code, Name: name}
	if err := g.Validate(); err != nil {
		return domain.Graph{}, err
	}
	var out domain.Graph
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertGraph(ctx, code, name, authorityBearing)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "graph.create", "graph", created.ID, "", created)
	})
	return out, err
}

// UpdateGraph renames / promotes-default / flips authority-bearing (guarded), recording the action.
// command is locked authority-bearing (also a DB CHECK); the only default cannot be unset directly
// (promote another instead). The TRUE->FALSE authority flip's "no active subtree assignments" guard
// (D-DirectoryGraphs) is a no-op until assignments exist (M7).
func (s *Service) UpdateGraph(ctx context.Context, id string, patch domain.GraphPatch) (domain.Graph, error) {
	if patch.Name != nil && *patch.Name == "" {
		return domain.Graph{}, domain.ErrInvalidUnit
	}
	var out domain.Graph
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		g, err := repo.GetGraphByID(ctx, id)
		if err != nil {
			return err
		}
		if patch.IsAuthorityBearing != nil && !*patch.IsAuthorityBearing && g.Code == domain.CommandGraphCode {
			return domain.ErrGraphProtected
		}
		if patch.IsDefault != nil {
			if *patch.IsDefault {
				if err := repo.ClearDefaultGraphs(ctx); err != nil {
					return err
				}
			} else if g.IsDefault {
				return domain.ErrGraphProtected // a default must exist; promote another instead
			}
		}
		updated, err := repo.UpdateGraph(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "graph.update", "graph", id, "", updated)
	})
	return out, err
}

// DeleteGraph soft-deletes a graph, guarding the registry invariants: command is undeletable, the
// sole default cannot be deleted, at least one graph must remain, and a graph with live edges is in
// use (D-Graphs). (Active subtree-assignment guard arrives with M7.)
func (s *Service) DeleteGraph(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		g, err := repo.GetGraphByID(ctx, id)
		if err != nil {
			return err
		}
		if g.Code == domain.CommandGraphCode || g.IsDefault {
			return domain.ErrGraphProtected
		}
		count, err := repo.CountActiveGraphs(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return domain.ErrGraphProtected
		}
		hasEdges, err := repo.GraphHasLiveEdges(ctx, g.ID)
		if err != nil {
			return err
		}
		if hasEdges {
			return domain.ErrGraphInUse
		}
		if err := repo.SoftDeleteGraph(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "graph.delete", "graph", id, "", map[string]string{"code": g.Code})
	})
}

// ---------------------------------------------------------------- helpers

// resolveGraphs returns the single graph named by code, or all graphs when code is nil.
func (s *Service) resolveGraphs(ctx context.Context, graphCode *string) ([]domain.Graph, error) {
	repo := s.newRepo(s.pool)
	if graphCode != nil {
		g, err := repo.GetGraphByCode(ctx, *graphCode)
		if err != nil {
			return nil, err
		}
		return []domain.Graph{g}, nil
	}
	return repo.ListGraphs(ctx)
}

// singleTargetID returns the lone graph's RID when exactly one graph is in scope, else "" (a
// multi-graph audit row carries no single target).
func singleTargetID(graphs []domain.Graph) string {
	if len(graphs) == 1 {
		return graphs[0].ID
	}
	return ""
}

func defaultGraph(code string) string {
	if code == "" {
		return domain.CommandGraphCode
	}
	return code
}

func resolvePageSize(requested int) int {
	if requested <= 0 {
		return DefaultPageSize
	}
	if requested > MaxPageSize {
		return MaxPageSize
	}
	return requested
}

// encodeCursor/decodeCursor make the keyset position (the last row's RID) into an opaque,
// URL-safe page token (API conventions: token pagination, no offsets).
func encodeCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeCursor(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// inTx runs fn in a transaction, committing on success and rolling back on error (the deferred
// rollback after a successful commit is a no-op).
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
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID, unitID string, after any) error {
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
		UnitID:     unitID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID composes the Action RID via the same SQL generator every module uses, so the audit
// log's action__<type> RID-shape CHECK is satisfied. The action code (e.g. "unit.edge.add") becomes
// the entity_type slot "action__unit_edge_add".
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	entityType := "action__" + sanitizeAction(action)
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_rid('tenant', $1)", entityType).Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func sanitizeAction(action string) string {
	b := make([]byte, len(action))
	for i := 0; i < len(action); i++ {
		if action[i] == '.' {
			b[i] = '_'
		} else {
			b[i] = action[i]
		}
	}
	return string(b)
}

// requestID is the correlation key shared with logs/metrics/traces: the request's trace id, with a
// generated fallback for out-of-request callers (e.g. integration tests) so the audit Entry and
// lifecycle event always have a non-empty requestId.
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
