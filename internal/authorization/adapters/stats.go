// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/authorization/adapters/authzsql"
	"github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// The assignment dashboard aggregate (M58 ticket 6 / D-ObjectFacets): every selected facet's
// distribution plus the total, over the same candidate set ListAssignments pages under the same
// filter.
//
// TWO arms, {instance-admin, reach-scoped}, and no search twin — a grant has no name of its own, so
// authz_role_assignments carries no search_text haystack for an R-21 split to split.
//
// The scoped arm uses the reach SET form only, with no dense twin. A scoped AGGREGATE, unlike a
// scoped LIST, has no early-terminating LIMIT to lose: it visits every candidate row whatever the
// reach size, so the point probe's dense-reach advantage does not arise (M57's measurement) and a
// second plan would be two shapes for one answer.

// AssignmentStats answers the whole dashboard in one statement. readerPersonID empty is the
// instance-admin arm, which carries no reach predicate at all — which is why it is a separate query
// rather than the scoped one with a flag.
func (r *Repository) AssignmentStats(ctx context.Context, f domain.AssignmentFilter, readerPersonID string, sel stats.Selection) ([]stats.Group, error) {
	w := assignmentStatsWants(sel)
	if readerPersonID == "" {
		rows, err := r.q.AssignmentStats(ctx, authzsql.AssignmentStatsParams{
			SubjectPersonID:     textPtr(f.SubjectPersonID),
			TargetUnitID:        textPtr(f.TargetUnitID),
			RoleID:              textPtr(f.RoleID),
			Scope:               textPtr(f.Scope),
			GraphID:             textPtr(f.GraphID),
			TopN:                int32(sel.TopN()),
			WantSubjectPersonID: w.subjectPersonID,
			WantRoleID:          w.roleID,
			WantTargetUnitID:    w.targetUnitID,
			WantScope:           w.scope,
			WantGraphID:         w.graphID,
		})
		if err != nil {
			return nil, err
		}
		return assignmentStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	}
	rows, err := r.q.AssignmentStatsForSubject(ctx, authzsql.AssignmentStatsForSubjectParams{
		SubjectPersonID:     textPtr(f.SubjectPersonID),
		TargetUnitID:        textPtr(f.TargetUnitID),
		RoleID:              textPtr(f.RoleID),
		Scope:               textPtr(f.Scope),
		GraphID:             textPtr(f.GraphID),
		ReaderPersonID:      readerPersonID,
		Permission:          PermAssignmentReadCode,
		TopN:                int32(sel.TopN()),
		WantSubjectPersonID: w.subjectPersonID,
		WantRoleID:          w.roleID,
		WantTargetUnitID:    w.targetUnitID,
		WantScope:           w.scope,
		WantGraphID:         w.graphID,
	})
	if err != nil {
		return nil, err
	}
	return assignmentStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
		return rows[i].Facet, rows[i].Bucket, rows[i].N
	}), nil
}

// assignmentStatsWants projects a selection onto the per-branch flags. An unselected facet's branch is
// a one-time false filter the planner skips, so it is never grouped.
type assignmentStatsWantFlags struct {
	subjectPersonID, roleID, targetUnitID, scope, graphID bool
}

func assignmentStatsWants(sel stats.Selection) assignmentStatsWantFlags {
	return assignmentStatsWantFlags{
		subjectPersonID: sel.Wants("subjectPersonId"),
		roleID:          sel.Wants("roleId"),
		targetUnitID:    sel.Wants("targetUnitId"),
		scope:           sel.Wants("scope"),
		graphID:         sel.Wants("graphId"),
	}
}

// assignmentStatsGroups maps the raw aggregate rows; a NULL bucket stays NULL (the (unknown) bucket).
// Both arms return row types with identical fields, so the accessor is passed in rather than the
// slice — one mapping, two callers, the same reason the aggregate SQL is one block.
func assignmentStatsGroups(n int, at func(int) (string, pgtype.Text, int64)) []stats.Group {
	out := make([]stats.Group, 0, n)
	for i := 0; i < n; i++ {
		facetKey, bucket, count := at(i)
		g := stats.Group{Facet: facetKey, Count: count}
		if bucket.Valid {
			k := bucket.String
			g.Key = &k
		}
		out = append(out, g)
	}
	return out
}
