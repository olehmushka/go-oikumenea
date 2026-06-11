package domain

import (
	"context"
	"strings"
)

// ActiveGrant is one active assignment with its role's resolved permission set — the PDP's per-grant
// input (assembled by the application from the repository). GraphID/GraphCode are empty for unit
// scope. Perms is the role's full code membership (already validated against the catalog).
type ActiveGrant struct {
	AssignmentID string
	RoleID       string
	RoleCode     string
	TargetUnitID string
	Scope        Scope
	GraphID      string
	GraphCode    string
	Perms        map[Permission]struct{}
}

// Has reports whether the grant's role carries permission p.
func (g ActiveGrant) Has(p Permission) bool {
	_, ok := g.Perms[p]
	return ok
}

// Contribution names one reason an ALLOW decision was reached (decision-explain, DS-16). For an
// instance-plane allow only InstanceAdmin is set; otherwise the contributing assignment is named.
type Contribution struct {
	InstanceAdmin bool
	AssignmentID  string
	RoleCode      string
	TargetUnitID  string
	Scope         Scope
	GraphCode     string
}

// Decision is the PDP's answer. Allow is the verdict; Via (ALLOW) or DenyReason (DENY) carry the
// explanation when explain is requested. Via may name several contributions (union across graphs).
type Decision struct {
	Allow      bool
	Via        []Contribution
	DenyReason string
}

// DecisionInput is one authorize question plus the subject's already-fetched authority state. The
// application fetches Grants + IsInstanceAdmin once and may reuse them across a batch.
type DecisionInput struct {
	Grants          []ActiveGrant
	IsInstanceAdmin bool
	Action          string
	UnitID          string
	Explain         bool
}

// PDP is the Policy Decision Point engine. It is pure logic over the supplied authority state plus
// the tenant closure port; it reads no rank, no position, and no per-request state beyond its inputs
// (decisions are pure functions of current data — assignments + closure + code catalog).
type PDP struct {
	closure ClosurePort
}

// NewPDP builds the engine over the tenant closure port.
func NewPDP(closure ClosurePort) PDP { return PDP{closure: closure} }

// Decide answers authorize(person, action, unit). The algorithm (docs/modules/authorization.md):
//
//  1. An active instance admin is allowed everything (the cluster-admin plane). This is the reading
//     the bootstrap requires — the first admin must be able to grant unit assignments (assignment.grant
//     is unit-scoped) before any unit assignment exists, so the instance plane cannot be limited to
//     only instance-scope permissions or the system could never delegate. See the module note.
//  2. Instance-scope permissions are satisfiable ONLY on the instance plane (roles never carry them),
//     so a non-admin is denied any instance-scope action outright.
//  3. Otherwise the action must appear in some active grant that REACHES unitID: a `unit` grant whose
//     target is unitID, or a `subtree` grant on an authority-bearing graph whose target is unitID or
//     an ancestor of it in that graph's closure. Union across graphs.
func (p PDP) Decide(ctx context.Context, in DecisionInput) (Decision, error) {
	if in.IsInstanceAdmin {
		return allow(in.Explain, Contribution{InstanceAdmin: true}), nil
	}
	if IsInstanceScope(in.Action) {
		return deny(in.Explain, "action is instance-scope and the subject is not an instance admin"), nil
	}

	action := Permission(in.Action)
	authority := map[string]bool{} // graphID -> is_authority_bearing (memoized within this decision)
	var via []Contribution

	for _, g := range in.Grants {
		if !g.Has(action) {
			continue
		}
		switch g.Scope {
		case ScopeUnit:
			if g.TargetUnitID == in.UnitID {
				if !in.Explain {
					return allow(false, Contribution{}), nil
				}
				via = append(via, contributionOf(g))
			}
		case ScopeSubtree:
			bearing, ok := authority[g.GraphID]
			if !ok {
				b, err := p.closure.IsAuthorityBearing(ctx, g.GraphID)
				if err != nil {
					return Decision{}, err
				}
				bearing = b
				authority[g.GraphID] = b
			}
			if !bearing {
				continue // directory-only graph cascades nothing (D-DirectoryGraphs)
			}
			reaches := g.TargetUnitID == in.UnitID // self: a subtree includes its root
			if !reaches {
				r, err := p.closure.IsAncestorOrSelf(ctx, g.GraphID, g.TargetUnitID, in.UnitID)
				if err != nil {
					return Decision{}, err
				}
				reaches = r
			}
			if reaches {
				if !in.Explain {
					return allow(false, Contribution{}), nil
				}
				via = append(via, contributionOf(g))
			}
		}
	}

	if len(via) > 0 {
		return Decision{Allow: true, Via: via}, nil
	}
	return deny(in.Explain, "the requested permission is not in the subject's effective set for this unit"), nil
}

// Reach is the subject's effective read/write unit-set (D-RLSDefenseInDepth / D-PersonReadScope): the
// units a subject may read / write, used by the shadow gate and (later) the RLS backstop GUCs. For an
// instance admin the InstanceAdmin flag is set and the sets are left empty (the plane covers all).
type Reach struct {
	InstanceAdmin bool
	Readable      map[string]struct{}
	Writable      map[string]struct{}
}

// Reachable reports whether the reach permits reading unitID (instance plane reads everything).
func (r Reach) Reachable(unitID string) bool {
	if r.InstanceAdmin {
		return true
	}
	_, ok := r.Readable[unitID]
	return ok
}

// ReachSet expands the subject's active grants into the effective read/write unit-sets: each grant's
// target (always), plus the authority-bearing subtree descendants for a subtree grant; classified
// into Readable / Writable by whether the role carries any read / any mutating permission.
func (p PDP) ReachSet(ctx context.Context, grants []ActiveGrant, isInstanceAdmin bool) (Reach, error) {
	if isInstanceAdmin {
		return Reach{InstanceAdmin: true, Readable: map[string]struct{}{}, Writable: map[string]struct{}{}}, nil
	}
	out := Reach{Readable: map[string]struct{}{}, Writable: map[string]struct{}{}}
	for _, g := range grants {
		hasRead, hasWrite := classify(g.Perms)
		if !hasRead && !hasWrite {
			continue
		}
		units := []string{g.TargetUnitID}
		if g.Scope == ScopeSubtree {
			bearing, err := p.closure.IsAuthorityBearing(ctx, g.GraphID)
			if err != nil {
				return Reach{}, err
			}
			if bearing {
				desc, err := p.closure.DescendantUnitIDs(ctx, g.GraphID, g.TargetUnitID)
				if err != nil {
					return Reach{}, err
				}
				units = append(units, desc...)
			}
		}
		for _, u := range units {
			if hasRead {
				out.Readable[u] = struct{}{}
			}
			if hasWrite {
				out.Writable[u] = struct{}{}
			}
		}
	}
	return out, nil
}

// classify partitions a role's permission set into read-bearing / write-bearing. A `*.read`
// permission is read-bearing; any other (catalog) permission is a mutating/write permission. (The
// instance-scope permissions never appear in a role, so they are not considered here.)
func classify(perms map[Permission]struct{}) (hasRead, hasWrite bool) {
	for p := range perms {
		if isReadPermission(p) {
			hasRead = true
		} else {
			hasWrite = true
		}
	}
	return
}

// isReadPermission reports whether a permission is a read (the `*.read` family the shadow gate
// consults). All read permissions in the catalog end in ".read".
func isReadPermission(p Permission) bool { return strings.HasSuffix(string(p), ".read") }

func contributionOf(g ActiveGrant) Contribution {
	return Contribution{
		AssignmentID: g.AssignmentID,
		RoleCode:     g.RoleCode,
		TargetUnitID: g.TargetUnitID,
		Scope:        g.Scope,
		GraphCode:    g.GraphCode,
	}
}

func allow(explain bool, c Contribution) Decision {
	d := Decision{Allow: true}
	if explain {
		d.Via = []Contribution{c}
	}
	return d
}

func deny(explain bool, reason string) Decision {
	d := Decision{Allow: false}
	if explain {
		d.DenyReason = reason
	}
	return d
}

// ShadowGate filters a set of candidate units to those the subject may see: a `public` unit passes
// (subject to the caller's normal read permission, checked separately); a `shadow` unit passes only
// if it is in the subject's readable reach (D-Shadow gate). Returns the allowed unit ids as a set.
//
// It is a pure second pass owned here, reached via application.FilterVisibleUnits from tenant's
// list/ancestors/descendants reads (F-002 A-lite). `shadow` reports, per candidate unit id, whether
// that unit is shadow (true) or public (false).
func ShadowGate(reach Reach, candidates []string, shadow map[string]bool) map[string]struct{} {
	allowed := make(map[string]struct{}, len(candidates))
	for _, u := range candidates {
		if shadow[u] && !reach.Reachable(u) {
			continue // hidden: a shadow unit the subject's *.read does not reach
		}
		allowed[u] = struct{}{}
	}
	return allowed
}
