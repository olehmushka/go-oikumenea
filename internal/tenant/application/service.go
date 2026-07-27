// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application holds the tenant module's application service — the orchestrator the transport
// layer calls to read/mutate the unit graph, and that maintains the per-graph closure and records
// audit rows in the same transaction as each write (D-Audit / D-ClosureIntegrity). It depends on
// the domain port, the platform DB surface, and the audit service; it never imports the adapters
// package directly (the repository factory is injected by module.go).
package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/pkg/metrics"
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

// metricClosureEditSeconds times an incremental closure edit, tagged op=add|remove (architecture
// review R-20). The M48 property is "cost ∝ affected slice, not graph size"; this latency is the
// live proxy for it. A slice-row-count histogram would show the slice directly but the repo's
// Extend/ShrinkClosureForEdge return only error today, so it is deferred rather than reshape the
// sqlc queries (docs/modules/platform.md).
const metricClosureEditSeconds = "tenant.closure.edit_seconds"

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
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		// The owning organization must exist; the unit's domain defaults to the org's domain when
		// omitted (mixed-domain trees are allowed, so an explicit domain may differ — M40).
		org, err := repo.GetOrganization(ctx, u.OrgID)
		if err != nil {
			return err
		}
		if u.DomainID == "" {
			u.DomainID = org.DomainID
		}
		// A kind, if given, must belong to the unit's domain (domain-scoped catalog).
		if u.KindID != nil {
			k, err := repo.GetUnitKind(ctx, *u.KindID)
			if err != nil {
				return err
			}
			if k.DomainID != u.DomainID {
				return domain.ErrUnitKindNotFound
			}
		}
		if err := u.Validate(); err != nil {
			return err
		}
		created, err := repo.InsertUnit(ctx, u)
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
	return s.newRepo(s.querier(ctx)).GetUnit(ctx, id)
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

// UpsertUnitLanguage adds (or flips the official flag of) a unit's official/working language
// (D-Languages, M18; keyed on unit+language). The unit must exist; the languoid existence is enforced
// by the FK (a violation surfaces as ErrUnknownLanguage). Returns the stored row joined to the name.
func (s *Service) UpsertUnitLanguage(ctx context.Context, l domain.UnitLanguage) (domain.UnitLanguage, error) {
	if err := l.Validate(); err != nil {
		return domain.UnitLanguage{}, err
	}
	var out domain.UnitLanguage
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetUnit(ctx, l.UnitID); err != nil {
			return err
		}
		_, err := repo.GetUnitLanguage(ctx, l.UnitID, l.LanguageID)
		switch {
		case err == nil:
			if err := repo.UpdateUnitLanguage(ctx, l); err != nil {
				return err
			}
		case errors.Is(err, domain.ErrUnitLanguageNotFound):
			if err := repo.InsertUnitLanguage(ctx, l); err != nil {
				return err
			}
		default:
			return err
		}
		saved, err := repo.GetUnitLanguage(ctx, l.UnitID, l.LanguageID)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "unit.language.upsert", "unit", l.UnitID, l.UnitID, map[string]any{"unitId": l.UnitID, "languageId": l.LanguageID})
	})
	return out, err
}

// DeleteUnitLanguage removes a unit's language by languoid id.
func (s *Service) DeleteUnitLanguage(ctx context.Context, unitID, languageID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.DeleteUnitLanguage(ctx, unitID, languageID); err != nil {
			return err
		}
		return s.record(ctx, tx, "unit.language.delete", "unit", unitID, unitID, map[string]any{"unitId": unitID, "languageId": languageID})
	})
}

// ListUnitLanguages lists a unit's official/working languages (the unit must exist).
func (s *Service) ListUnitLanguages(ctx context.Context, unitID string) ([]domain.UnitLanguage, error) {
	repo := s.newRepo(s.querier(ctx))
	if _, err := repo.GetUnit(ctx, unitID); err != nil {
		return nil, err
	}
	return repo.ListUnitLanguages(ctx, unitID)
}

// ListUnits returns a keyset-paginated page of units within an organization (REQUIRED orgID;
// D-TenantOrganizations, M40). For the flat listing it is optionally filtered by domain/kind/level.
// For hierarchical browsing in graph graphCode (default command) it returns either the org's root
// units (rootsOnly) or one unit's DIRECT children (parent); those two are mutually exclusive and
// ignore the domain/kind/level filters.
func (s *Service) ListUnits(ctx context.Context, orgID string, domainID, kindID *string, level *int, graphCode string, parent *string, rootsOnly bool, pageSize int, pageToken string) (UnitPage, error) {
	if orgID == "" {
		return UnitPage{}, domain.ErrInvalidUnit
	}
	if parent != nil && rootsOnly {
		return UnitPage{}, domain.ErrInvalidUnit
	}
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return UnitPage{}, err
	}
	repo := s.newRepo(s.querier(ctx))

	var units []domain.Unit
	switch {
	case parent != nil:
		// Direct children of the parent unit in the graph; resolve the graph from the parent's org.
		p, err := repo.GetUnit(ctx, *parent)
		if err != nil {
			return UnitPage{}, err
		}
		g, err := repo.GetGraphForOrgByCode(ctx, &p.OrgID, defaultGraph(graphCode))
		if err != nil {
			return UnitPage{}, err
		}
		units, err = repo.ListChildUnits(ctx, *parent, g.ID, after, size+1)
		if err != nil {
			return UnitPage{}, err
		}
	case rootsOnly:
		g, err := repo.GetGraphForOrgByCode(ctx, &orgID, defaultGraph(graphCode))
		if err != nil {
			return UnitPage{}, err
		}
		units, err = repo.ListRootUnits(ctx, orgID, g.ID, after, size+1)
		if err != nil {
			return UnitPage{}, err
		}
	default:
		units, err = repo.ListUnits(ctx, orgID, domainID, kindID, level, after, size+1)
		if err != nil {
			return UnitPage{}, err
		}
	}

	if len(units) > size {
		last := units[size-1]
		return UnitPage{Units: units[:size], NextPageToken: listing.EncodeCursor(last.ID)}, nil
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

// SetUnitCode sets, corrects, or clears a unit's code (D-UnitCodeLifecycle, M28) — all in one
// transaction: validate the new shape, reject a collision with another active coded unit, update the
// row, append the append-only code-change event, and record the audit action. newCode nil clears the
// code (the unit becomes a non-separate sub-unit). A no-op (same code) still records an event so the
// ledger reflects every recode request; callers that want to skip no-ops can compare before calling.
func (s *Service) SetUnitCode(ctx context.Context, unitID string, newCode *string, reason string) (domain.Unit, error) {
	if err := domain.ValidateCode(newCode); err != nil {
		return domain.Unit{}, err
	}
	var out domain.Unit
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		current, err := repo.GetUnit(ctx, unitID)
		if err != nil {
			return err
		}
		if newCode != nil {
			n, err := repo.CountActiveUnitsByCode(ctx, *newCode, unitID)
			if err != nil {
				return err
			}
			if n > 0 {
				return domain.ErrUnitCodeConflict
			}
		}
		updated, err := repo.SetUnitCode(ctx, unitID, newCode)
		if err != nil {
			return err
		}
		if err := repo.InsertUnitCodeEvent(ctx, domain.UnitCodeEvent{
			UnitID:    unitID,
			OldCode:   current.Code,
			NewCode:   newCode,
			Reason:    reason,
			RequestID: requestID(ctx),
		}); err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "unit.recode", "unit", unitID, unitID, map[string]any{
			"oldCode": current.Code, "newCode": newCode, "reason": reason,
		})
	})
	return out, err
}

// ListUnitCodeEvents returns a unit's code-change history, newest first (the unit must exist).
func (s *Service) ListUnitCodeEvents(ctx context.Context, unitID string) ([]domain.UnitCodeEvent, error) {
	repo := s.newRepo(s.querier(ctx))
	if _, err := repo.GetUnit(ctx, unitID); err != nil {
		return nil, err
	}
	return repo.ListUnitCodeEvents(ctx, unitID)
}

// ---------------------------------------------------------------- edges

// AddEdge attaches childID as a child of parentID within a graph (default command), guarding
// against cycles, then incrementally extends the graph's closure (M48) and records the action —
// all in one transaction. graphCode "" resolves to the command graph. The per-graph closure lock
// is taken before the cycle guard: two concurrent guard-then-insert attaches could otherwise each
// pass the guard and jointly close a cycle.
func (s *Service) AddEdge(ctx context.Context, childID, parentID, graphCode string) (domain.Edge, error) {
	graphCode = defaultGraph(graphCode)
	if parentID == childID {
		return domain.Edge{}, domain.ErrUnitCycle // a self-loop is the degenerate cycle
	}
	var out domain.Edge
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		child, err := repo.GetUnit(ctx, childID)
		if err != nil {
			return err
		}
		if _, err := repo.GetUnit(ctx, parentID); err != nil {
			return err
		}
		// Resolve the graph within the child unit's organization, falling back to an instance-global
		// graph (e.g. religion's taxonomy graphs) — D-TenantOrganizations, M40.
		g, err := repo.GetGraphForOrgByCode(ctx, &child.OrgID, graphCode)
		if err != nil {
			return err
		}
		if err := repo.LockGraphForClosure(ctx, g.ID); err != nil {
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
		start := time.Now()
		if err := repo.ExtendClosureForEdge(ctx, g.ID, parentID, childID); err != nil {
			return err
		}
		metrics.FromContext(ctx).Timer(metricClosureEditSeconds, metrics.MustNewTag("op", "add")).UpdateSince(start)
		edge.Graph = g.Code
		out = edge
		return s.record(ctx, tx, "unit.edge.add", "unit", childID, childID, map[string]string{
			"graph": g.Code, "parentId": parentID, "childId": childID,
		})
	})
	return out, err
}

// RemoveEdge detaches childID from parentID within a graph (default command), incrementally
// shrinks the graph's closure (M48), and records the action. Detaching an absent edge is a no-op
// (idempotent) — and skipping the shrink then is load-bearing: without the edge, acyclicity does
// not rule out a child->parent path, which would break the slice algebra's assumptions.
func (s *Service) RemoveEdge(ctx context.Context, childID, parentID, graphCode string) error {
	graphCode = defaultGraph(graphCode)
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		child, err := repo.GetUnit(ctx, childID)
		if err != nil {
			return err
		}
		g, err := repo.GetGraphForOrgByCode(ctx, &child.OrgID, graphCode)
		if err != nil {
			return err
		}
		if err := repo.LockGraphForClosure(ctx, g.ID); err != nil {
			return err
		}
		deleted, err := repo.DeleteEdge(ctx, g.ID, parentID, childID)
		if err != nil {
			return err
		}
		if deleted > 0 {
			start := time.Now()
			if err := repo.ShrinkClosureForEdge(ctx, g.ID, parentID, childID); err != nil {
				return err
			}
			metrics.FromContext(ctx).Timer(metricClosureEditSeconds, metrics.MustNewTag("op", "remove")).UpdateSince(start)
		}
		return s.record(ctx, tx, "unit.edge.remove", "unit", childID, childID, map[string]string{
			"graph": g.Code, "parentId": parentID, "childId": childID,
		})
	})
}

// Ancestors returns the unit's ancestors in graph graphCode (default command), nearest first.
func (s *Service) Ancestors(ctx context.Context, unitID, graphCode string) ([]domain.UnitRef, error) {
	repo := s.newRepo(s.querier(ctx))
	unit, err := repo.GetUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	g, err := repo.GetGraphForOrgByCode(ctx, &unit.OrgID, defaultGraph(graphCode))
	if err != nil {
		return nil, err
	}
	return repo.ListAncestors(ctx, g.ID, unitID)
}

// Descendants returns a keyset-paginated page of the unit's subtree in graph graphCode (default
// command).
func (s *Service) Descendants(ctx context.Context, unitID, graphCode string, pageSize int, pageToken string) (UnitRefPage, error) {
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return UnitRefPage{}, err
	}
	repo := s.newRepo(s.querier(ctx))
	unit, err := repo.GetUnit(ctx, unitID)
	if err != nil {
		return UnitRefPage{}, err
	}
	g, err := repo.GetGraphForOrgByCode(ctx, &unit.OrgID, defaultGraph(graphCode))
	if err != nil {
		return UnitRefPage{}, err
	}
	refs, err := repo.ListDescendants(ctx, g.ID, unitID, after, size+1)
	if err != nil {
		return UnitRefPage{}, err
	}
	if len(refs) > size {
		last := refs[size-1]
		return UnitRefPage{Refs: refs[:size], NextPageToken: listing.EncodeCursor(last.ID)}, nil
	}
	return UnitRefPage{Refs: refs}, nil
}

// ---------------------------------------------------------------- closure integrity

// VerifyClosure diffs the stored closure vs. the edges per graph and upserts the per-graph drift
// status the closure-drift health reporter reads (default: all graphs). One transaction.
// Deliberately does not take the per-graph closure lock: it is read-only, and a transient false
// drift while an edit commits concurrently was always possible and is acceptable for a health probe.
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
			if err := repo.LockGraphForClosure(ctx, g.ID); err != nil {
				return err
			}
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

// ListGraphs returns an organization's graph registry plus the instance-global graphs (orgID nil =
// only the global graphs) in display order — D-TenantOrganizations, M40.
func (s *Service) ListGraphs(ctx context.Context, orgID *string) ([]domain.Graph, error) {
	if orgID != nil {
		if _, err := s.newRepo(s.querier(ctx)).GetOrganization(ctx, *orgID); err != nil {
			return nil, err
		}
	}
	return s.newRepo(s.querier(ctx)).ListGraphsForOrg(ctx, orgID)
}

// AddGraph validates and adds a graph to an organization (orgID nil = an instance-global graph) and
// records the action. New graphs are never the default (promote via UpdateGraph).
func (s *Service) AddGraph(ctx context.Context, orgID *string, code, name string, authorityBearing bool) (domain.Graph, error) {
	g := domain.Graph{Code: code, Name: name}
	if err := g.Validate(); err != nil {
		return domain.Graph{}, err
	}
	var out domain.Graph
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if orgID != nil {
			if _, err := repo.GetOrganization(ctx, *orgID); err != nil {
				return err
			}
		}
		created, err := repo.InsertGraph(ctx, orgID, code, name, false, authorityBearing)
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
				if err := repo.ClearDefaultGraphsForOrg(ctx, g.OrgID); err != nil {
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
		count, err := repo.CountActiveGraphsForOrg(ctx, g.OrgID)
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

// resolveGraphs returns all graphs (across every organization and the global graphs) for the closure
// verify/rebuild diagnostics, optionally filtered to those whose code matches graphCode. Graph codes
// are no longer globally unique (per-org), so a code filter may match several graphs — M40.
func (s *Service) resolveGraphs(ctx context.Context, graphCode *string) ([]domain.Graph, error) {
	repo := s.newRepo(s.querier(ctx))
	ids, err := repo.ListGraphIDs(ctx)
	if err != nil {
		return nil, err
	}
	graphs := make([]domain.Graph, 0, len(ids))
	for _, id := range ids {
		g, err := repo.GetGraphByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if graphCode == nil || g.Code == *graphCode {
			graphs = append(graphs, g)
		}
	}
	return graphs, nil
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

// querier returns the request-pinned RLS connection if one is in context (every authenticated request
// pins one — db.AcquireScoped/WithConn), else the bare pool. Reads/writes on unit-scoped tables MUST go
// through it so the app.* RLS GUCs apply (D-RLSDefenseInDepth); a write begun on a non-pinned pool
// connection would have empty writable_units and fail the policy's WITH CHECK.
func (s *Service) querier(ctx context.Context) db.Querier {
	return db.RequestQuerier(ctx, s.pool)
}

// inTx runs fn in a transaction, committing on success and rolling back on error (the deferred
// rollback after a successful commit is a no-op).
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.querier(ctx).Begin(ctx)
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

// mintActionRID mints an Action RID (tenant service=4, kind=action=3, generic action type=0).
// The specific action name is recorded separately in audit_log.action (D-Audit).
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	_ = action
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(4, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
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
