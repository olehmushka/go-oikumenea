// Package domain holds the tenant module's pure logic: units (nodes), graphs (named hierarchies),
// the parent->child edge, the derived closure, lifecycle transitions, their invariants, and the
// Repository port it needs from the outside world (overview.md layering). No I/O, no framework
// imports — only the standard library. Tenant owns the organization as a multi-parent, multi-root
// DAG per graph (D-Graphs / docs/modules/tenant.md); the M7 PDP reads its closure. The module never
// decides access — it calls the PDP.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB
// constraints enforce the same shapes as a backstop.
var (
	ErrUnitNotFound      = errors.New("unit not found")
	ErrUnitCodeConflict  = errors.New("unit code already exists")
	ErrUnitCycle         = errors.New("edge would create a cycle in its graph")
	ErrEdgeExists        = errors.New("edge already exists in this graph")
	ErrInvalidUnit       = errors.New("invalid unit")
	ErrInvalidTransition = errors.New("invalid lifecycle transition")
	ErrGraphNotFound     = errors.New("graph not found")
	ErrGraphCodeConflict = errors.New("graph code already exists")
	ErrGraphInUse        = errors.New("graph is in use")
	ErrGraphProtected    = errors.New("graph is protected")
)

// CommandGraphCode is the seeded default + undeletable + locked-authority-bearing graph (D-Graphs).
const CommandGraphCode = "command"

// Visibility is a unit's read-time public/shadow gate value (the shadow gate lands in M7).
type Visibility string

const (
	VisibilityPublic Visibility = "public"
	VisibilityShadow Visibility = "shadow"
)

// State is a unit's lifecycle state (reversible; transitions recorded as append-only events).
type State string

const (
	StateActive    State = "active"
	StateSuspended State = "suspended"
	StateArchived  State = "archived"
)

// Unit is a node in the org graph. Name is the default-locale text (the i18n store holds the other
// locales; the transport assembles the response map). UnitKind/Level are directory attributes only
// — never PDP or shadow-gate inputs.
type Unit struct {
	ID         string
	Code       string
	Name       string
	UnitKind   string // "" = none
	Level      *int   // nil = unset
	Visibility Visibility
	State      State
	Metadata   json.RawMessage
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UnitPatch is a partial update of a unit: a nil field leaves the stored value unchanged. Code is
// immutable by convention and not patchable.
type UnitPatch struct {
	Name       *string
	UnitKind   *string
	Level      *int
	Visibility *Visibility
	Metadata   json.RawMessage // nil = unchanged
}

// Graph is a named hierarchy over the units (D-Graphs). Each graph is independently a DAG.
type Graph struct {
	ID                 string
	Code               string
	Name               string
	IsDefault          bool
	IsAuthorityBearing bool
}

// GraphPatch is a partial update of a graph: a nil field leaves the stored value unchanged.
type GraphPatch struct {
	Name               *string
	IsDefault          *bool
	IsAuthorityBearing *bool
}

// Edge is a directed parent->child relationship within one graph (link__parent_of). Graph is the
// graph's code (the application resolves it from the RID for the response).
type Edge struct {
	ID        string
	Graph     string
	ParentID  string
	ChildID   string
	CreatedAt time.Time
}

// UnitRef is a lightweight unit reference with its closure depth (ancestor/descendant listings).
type UnitRef struct {
	ID         string
	Code       string
	Name       string
	Depth      int
	Visibility Visibility // public/shadow, for the read-time shadow-visibility gate
}

// ClosureReport is the result of a closure verify/rebuild for one graph (D-ClosureIntegrity).
type ClosureReport struct {
	Graph        string
	MissingCount int
	ExtraCount   int
	InDrift      bool
	Sample       json.RawMessage
}

// Validate enforces the unit invariants before insert: a non-empty, whitespace-free code, a
// non-empty name, and a known visibility.
func (u Unit) Validate() error {
	if !validCode(u.Code) {
		return wrapInvalid("code must be non-empty and contain no whitespace")
	}
	if strings.TrimSpace(u.Name) == "" {
		return wrapInvalid("name is required")
	}
	if u.Visibility != VisibilityPublic && u.Visibility != VisibilityShadow {
		return wrapInvalid("visibility must be public or shadow")
	}
	return nil
}

// Validate enforces the graph invariants before insert: a non-empty, whitespace-free code and a
// non-empty name. The command graph is seeded (not created via this path).
func (g Graph) Validate() error {
	if !validCode(g.Code) {
		return wrapInvalid("code must be non-empty and contain no whitespace")
	}
	if strings.TrimSpace(g.Name) == "" {
		return wrapInvalid("name is required")
	}
	return nil
}

// CanTransitionTo reports whether a unit may move from its current state to `to` (tenant.md:
// suspend/archive/restore; reversible). A no-op (same state) is not a valid transition.
func (s State) CanTransitionTo(to State) bool {
	switch s {
	case StateActive:
		return to == StateSuspended || to == StateArchived
	case StateSuspended:
		return to == StateActive || to == StateArchived
	case StateArchived:
		return to == StateActive // restore
	default:
		return false
	}
}

// ValidState reports whether s is one of the known lifecycle states.
func ValidState(s State) bool {
	return s == StateActive || s == StateSuspended || s == StateArchived
}

func wrapInvalid(msg string) error { return errors.Join(ErrInvalidUnit, errors.New(msg)) }

// validCode is the shared code shape guard: non-empty, <=128 chars, no whitespace (D-Code:
// operator-assigned, locale-agnostic, immutable by convention).
func validCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	return !strings.ContainsFunc(code, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// Repository is the persistence port the tenant application service depends on; the pgx/sqlc
// adapter implements it. Each method runs on whatever DBTX the adapter was constructed with, so a
// write + its closure recompute + its audit row share one transaction (D-Audit / D-ClosureIntegrity).
type Repository interface {
	// units
	InsertUnit(ctx context.Context, u Unit) (Unit, error)
	GetUnit(ctx context.Context, id string) (Unit, error)
	UpdateUnit(ctx context.Context, id string, patch UnitPatch) (Unit, error)
	SetUnitState(ctx context.Context, id string, state State) (Unit, error)
	ListUnits(ctx context.Context, level *int, after string, limit int) ([]Unit, error)

	// graphs
	InsertGraph(ctx context.Context, code, name string, authorityBearing bool) (Graph, error)
	GetGraphByID(ctx context.Context, id string) (Graph, error)
	GetGraphByCode(ctx context.Context, code string) (Graph, error)
	ListGraphs(ctx context.Context) ([]Graph, error)
	ClearDefaultGraphs(ctx context.Context) error
	UpdateGraph(ctx context.Context, id string, patch GraphPatch) (Graph, error)
	SoftDeleteGraph(ctx context.Context, id string) error
	CountActiveGraphs(ctx context.Context) (int, error)
	GraphHasLiveEdges(ctx context.Context, graphID string) (bool, error)

	// edges
	InsertEdge(ctx context.Context, graphID, parentID, childID, createdBy string) (Edge, error)
	DeleteEdge(ctx context.Context, graphID, parentID, childID string) (int64, error)

	// closure
	ClosureHasPath(ctx context.Context, graphID, ancestorID, descendantID string) (bool, error)
	RecomputeClosure(ctx context.Context, graphID string) error
	VerifyClosure(ctx context.Context, graphID string) (missing, extra int, sample json.RawMessage, err error)
	UpsertClosureStatus(ctx context.Context, graphID string, missing, extra int, inDrift bool, sample json.RawMessage) error
	ListAncestors(ctx context.Context, graphID, unitID string) ([]UnitRef, error)
	ListDescendants(ctx context.Context, graphID, unitID, after string, limit int) ([]UnitRef, error)

	// lifecycle
	InsertLifecycleEvent(ctx context.Context, unitID string, from, to State, reason, actorPersonID, requestID string) error
}
