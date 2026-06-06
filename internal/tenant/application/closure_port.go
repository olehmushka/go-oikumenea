package application

import "context"

// This file exposes the tenant graph closure to the authorization PDP as a cross-module query
// (overview.md): the PDP unions a subject's grants over these lookups. They are keyed by graph RID
// (matching authz_role_assignments.graph_id) except ResolveGraph, which the grant path uses to turn a
// graph CODE into its RID + authority-bearing flag. See docs/modules/authorization.md (PDP step 3).

// closurePortBatch bounds the descendant paging used to materialize a subtree's effective reach.
const closurePortBatch = 1000

// IsAncestorOrSelf reports whether ancestorUnitID reaches descendantUnitID in the graph's closure
// (one indexed lookup). The reflexive (self) row exists only for units that appear in the graph's
// edges, so the PDP additionally treats target==unit as self-authorized on its own.
func (s *Service) IsAncestorOrSelf(ctx context.Context, graphID, ancestorUnitID, descendantUnitID string) (bool, error) {
	return s.newRepo(s.pool).ClosureHasPath(ctx, graphID, ancestorUnitID, descendantUnitID)
}

// IsAuthorityBearing reports whether the graph cascades authority (D-DirectoryGraphs).
func (s *Service) IsAuthorityBearing(ctx context.Context, graphID string) (bool, error) {
	g, err := s.newRepo(s.pool).GetGraphByID(ctx, graphID)
	if err != nil {
		return false, err
	}
	return g.IsAuthorityBearing, nil
}

// DescendantUnitIDs returns the strict descendants (excludes self) of unitID in the graph's closure,
// used to expand a subtree grant into the effective read/write unit-set (D-RLSDefenseInDepth). It
// pages through the closure so the full subtree is returned. (Named to avoid colliding with the
// paginated Descendants endpoint helper.)
func (s *Service) DescendantUnitIDs(ctx context.Context, graphID, unitID string) ([]string, error) {
	repo := s.newRepo(s.pool)
	var out []string
	after := ""
	for {
		refs, err := repo.ListDescendants(ctx, graphID, unitID, after, closurePortBatch)
		if err != nil {
			return nil, err
		}
		for _, r := range refs {
			out = append(out, r.ID)
		}
		if len(refs) < closurePortBatch {
			break
		}
		after = refs[len(refs)-1].ID
	}
	return out, nil
}

// ResolveGraph turns a graph CODE into its RID and authority-bearing flag for the grant path. An
// empty code resolves to the registry default (command). Unknown codes surface ErrGraphNotFound.
func (s *Service) ResolveGraph(ctx context.Context, code string) (graphID string, authorityBearing bool, err error) {
	g, err := s.newRepo(s.pool).GetGraphByCode(ctx, defaultGraph(code))
	if err != nil {
		return "", false, err
	}
	return g.ID, g.IsAuthorityBearing, nil
}
