// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// `listAssignments` returns ACTIVE grants and keeps that default (decided M58 ticket 3, not
// re-litigated in ticket 6): an ended membership is ordinary directory history, while a revoked grant
// is a security artefact whose reachability is an authz read-surface decision rather than a
// facet-vocabulary one.
//
// The consequence is that `totalCount` counts active grants, not rows — which is fine while every
// surface agrees, and quietly wrong the moment one of them does not. The concrete failure: an
// aggregate that lost `revoked_at IS NULL` would count revoked grants into every bar while the list
// beside it kept omitting them, so the chart would sum to more than the list could ever show and the
// difference would look like a paging bug.
//
// There is no `active` facet to catch this the usual way — the whole point of the decision is that a
// distribution whose every row is active is a chart with one bar — so the predicate is pinned
// directly, in every query that describes the population.

// activeOnlyQueries are the surfaces that describe the active grant population: the three list plan
// shapes and the two aggregate arms. Writes are absent (a revoke must be able to see the row it
// revokes) and so is GetAssignment (a point read may legitimately return a revoked grant, which is
// why the console keeps a `status` column).
var activeOnlyQueries = []string{
	"ListAssignments",
	"ListAssignmentsForSubject",
	"ListAssignmentsForSubjectDense",
	"AssignmentStats",
	"AssignmentStatsForSubject",
}

func TestAssignmentSurfacesAreActiveOnly(t *testing.T) {
	body := readQueries(t)
	for _, name := range activeOnlyQueries {
		q := namedQuery(t, body, name)
		if !regexp.MustCompile(`revoked_at\s+IS\s+NULL`).MatchString(q) {
			t.Errorf("%s does not filter `revoked_at IS NULL`. The list and the dashboard must describe "+
				"ONE population: a query that admits revoked grants would make totalCount disagree with "+
				"the rows the caller can page, and the gap would read as a paging bug", name)
		}
	}
}

// TestNoAssignmentSurfaceFiltersExpiry pins the other half of "active", which is deliberately NOT
// applied here. A grant's `expires_at` is evaluated at DECISION time by the PDP (D-TimeBoundGrants,
// silent expiry) and not by the list — an expired grant is still a row an administrator must be able
// to see and revoke. Adding an expiry predicate to a listing would hide exactly the grants most worth
// looking at, and would do it silently.
func TestNoAssignmentSurfaceFiltersExpiry(t *testing.T) {
	body := readQueries(t)
	for _, name := range activeOnlyQueries {
		q := namedQuery(t, body, name)
		if regexp.MustCompile(`expires_at\s+(IS\s+NULL|>)`).MatchString(q) {
			t.Errorf("%s filters on expires_at. Expiry is a DECISION-time rule (D-TimeBoundGrants), not "+
				"a listing rule: an expired grant is still a row, and one an administrator most wants "+
				"to find", name)
		}
	}
}

// TestActiveOnlyQueriesCoverEveryDescribingSurface is the non-vacuity floor: a new list or aggregate
// arm with no entry above would go unchecked while both tests stayed green.
func TestActiveOnlyQueriesCoverEveryDescribingSurface(t *testing.T) {
	body := readQueries(t)
	listed := map[string]bool{}
	for _, n := range activeOnlyQueries {
		listed[n] = true
	}
	nameRe := regexp.MustCompile(`(?m)^-- name: (\w+) `)
	for _, m := range nameRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		describes := strings.HasPrefix(name, "ListAssignments") || strings.HasPrefix(name, "AssignmentStats")
		if describes && !listed[name] {
			t.Errorf("query %q lists or aggregates assignments but is not in activeOnlyQueries — add "+
				"it, so the population it describes is checked rather than assumed", name)
		}
	}
}

func readQueries(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("queries/authorization.sql")
	if err != nil {
		t.Fatalf("read queries/authorization.sql: %v", err)
	}
	return string(body)
}
