// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The assignment surfaces trim rows by REACH, and this file pins WHICH reach.
//
// Every other scoped list in the codebase asks `authz_readable_units(subject)` — "does the subject
// hold ANY '%.read' code on this unit". That is right where the endpoint has already checked its own
// read code and reach is only narrowing rows. On `listAssignments` it would WIDEN the surface:
// generic read-reach is a strict superset of assignment.read reach, so a caller holding, say,
// `person.read` over a unit and `assignment.read` somewhere unrelated would be handed grants that
// this endpoint's own per-unit arm has always refused them.
//
// Nothing about that is visible at a call site. `authz_readable_units_with(subject, permission)` and
// `authz_readable_units(subject)` differ by one argument, read alike, and the wrong one compiles,
// passes every other guard and returns MORE rows — which is exactly the shape of a change nobody
// notices. Hence two checks with no database: the queries name the narrow functions, and the Go that
// calls them passes the one permission code.

var narrowReachFns = []string{
	"oikumenea.authz_readable_units_with",
	"oikumenea.authz_unit_readable_with",
	"oikumenea.authz_readable_unit_count_with",
}

// assignmentReachQueries are the queries whose reach predicate must be the narrow one. The admin arms
// (ListAssignments, AssignmentStats) are deliberately absent: they carry NO reach predicate at all,
// and requiring one there would be requiring the wrong thing.
var assignmentReachQueries = []string{
	"ListAssignmentsForSubject",
	"ListAssignmentsForSubjectDense",
	"CountAssignmentReadableUnitsCapped",
	"AssignmentStatsForSubject",
}

// TestAssignmentReachAsksForOnePermission proves each reach-scoped assignment query calls one of the
// permission-parameterised reach functions and NOT the generic '%.read' family.
func TestAssignmentReachAsksForOnePermission(t *testing.T) {
	body, err := os.ReadFile("queries/authorization.sql")
	if err != nil {
		t.Fatalf("read queries/authorization.sql: %v", err)
	}
	for _, name := range assignmentReachQueries {
		q := namedQuery(t, string(body), name)
		if !containsAny(q, narrowReachFns) {
			t.Errorf("%s applies no narrow reach function (%s) — a reach-scoped assignment query that "+
				"does not ask for one permission is either ungated or gated on the wrong question",
				name, strings.Join(narrowReachFns, " / "))
		}
		// The generic family, spelled without the `_with` suffix. Checked by word boundary so
		// `authz_readable_units_with` does not match `authz_readable_units`.
		for _, generic := range []string{
			`oikumenea\.authz_readable_units\b`,
			`oikumenea\.authz_unit_readable_by\b`,
			`oikumenea\.authz_readable_unit_count\b`,
			`oikumenea\.authz_unit_in_reach\b`,
		} {
			if regexp.MustCompile(generic + `\s*\(`).MatchString(q) {
				t.Errorf("%s calls the GENERIC '%%.read' reach family (%s). That is a superset of "+
					"assignment.read reach: using it here would show grants the per-unit arm refuses, "+
					"so this endpoint would have got WIDER by acquiring a filter", name, generic)
			}
		}
	}
}

// TestAssignmentReachPassesTheAssignmentReadCode proves the Go side passes `assignment.read` and
// nothing else. The SQL check above cannot see this: the permission is a bind parameter, so a query
// naming the narrow function while its caller passed "person.read" — or "" — would pass every check
// in this file but the one below.
func TestAssignmentReachPassesTheAssignmentReadCode(t *testing.T) {
	if PermAssignmentReadCode != "assignment.read" {
		t.Fatalf("PermAssignmentReadCode is %q, want assignment.read", PermAssignmentReadCode)
	}
	fset := token.NewFileSet()
	for _, file := range []string{"repository.go", "stats.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Permission" {
				return true
			}
			id, ok := kv.Value.(*ast.Ident)
			if !ok || id.Name != "PermAssignmentReadCode" {
				t.Errorf("%s: a reach query is passed Permission: %s — every assignment reach call must "+
					"pass PermAssignmentReadCode, so the code the trim asks for is one edit in one place "+
					"rather than a literal per call site", file, exprText(kv.Value))
			}
			return true
		})
	}
}

// TestEveryScopedAssignmentQueryIsListed is the non-vacuity floor: a NEW reach-scoped query added to
// the module with no entry in assignmentReachQueries would simply go unchecked, and both tests above
// would stay green while the trim they exist to police went unpoliced.
func TestEveryScopedAssignmentQueryIsListed(t *testing.T) {
	body, err := os.ReadFile("queries/authorization.sql")
	if err != nil {
		t.Fatalf("read queries/authorization.sql: %v", err)
	}
	listed := map[string]bool{}
	for _, n := range assignmentReachQueries {
		listed[n] = true
	}
	nameRe := regexp.MustCompile(`(?m)^-- name: (\w+) `)
	for _, m := range nameRe.FindAllStringSubmatch(string(body), -1) {
		name := m[1]
		if !strings.Contains(name, "Assignment") {
			continue
		}
		q := namedQuery(t, string(body), name)
		if !containsAny(q, narrowReachFns) {
			continue // an admin arm or an unscoped write — nothing to trim
		}
		if !listed[name] {
			t.Errorf("query %q applies a reach predicate but is not listed in assignmentReachQueries — "+
				"add it, so the permission it asks for is checked rather than assumed", name)
		}
	}
}

// namedQuery returns the body of one `-- name: X` block, up to the next one.
func namedQuery(t *testing.T, body, name string) string {
	t.Helper()
	start := strings.Index(body, "-- name: "+name+" ")
	if start < 0 {
		t.Fatalf("no `-- name: %s` query in queries/authorization.sql", name)
	}
	rest := body[start+1:]
	if next := strings.Index(rest, "\n-- name: "); next >= 0 {
		return rest[:next]
	}
	return rest
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func exprText(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.SelectorExpr:
		return exprText(v.X) + "." + v.Sel.Name
	}
	return "<expr>"
}
