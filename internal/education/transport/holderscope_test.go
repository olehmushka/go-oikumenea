// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The holder read-scope gate (M58 ticket 7), pinned the way the shadow gate beside it is and for
// exactly the same reason it had to be.
//
// D-PersonReadScope has required a person's data to be readable only through that person's read
// scope since M0, and this module applied NO such scope anywhere until M58 ticket 7. Every
// person-binding read — enrollments, dorm stays, education appointments and the six reference-layer
// bindings — gated `education.read` ANYWHERE and then returned the rows, so ONE grant anywhere
// enumerated any person's education history instance-wide. The document module has had the probe
// since M56; the nine endpoints below simply never grew it.
//
// It went unnoticed for the reason the institution leak did: the gate lives PER HANDLER in the
// transport, so nothing outside this package can observe its absence. An application-layer test has
// no handler to lose it from, and would stay green while every one of these endpoints was reopened
// — which ticket 4 demonstrated by writing that test first and watching it pass through a
// reintroduced leak.
//
// So this asks a STRUCTURAL question — does the handler route through the one helper that owns the
// rule — rather than a behavioural one. The projection itself has moved before (it is the person
// service's SQL point probe, R-02.1) and will again; a handler that stops asking is the regression,
// and it is the same regression whatever the projection currently answers.
var holderScopedHandlers = []string{
	// EducationService
	"ListPersonEnrollments",
	"ListDormitoryStays",
	"ListPersonAppointments",
	// ReferenceService
	"ListPublicationAuthorships",
	"ListResearchMemberships",
	"ListGrantHoldings",
	"ListGovernanceMemberships",
	"ListQualificationAwards",
	"ListScholarshipAwards",
}

func TestHolderScopedHandlersProbeTheHolder(t *testing.T) {
	bodies := parseMethodBodies(t)
	if len(bodies) < 20 {
		t.Fatalf("parsed only %d methods — the parse is broken, so every check below is vacuous", len(bodies))
	}
	for _, name := range holderScopedHandlers {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("%s is not a method on this package's services — renamed or removed? A holder-scoped "+
				"read that disappears from the source must fail here rather than silently leave the list", name)
			continue
		}
		if !strings.Contains(body, "holderReadable ") {
			t.Errorf("%s does not probe holderReadable — it returns one named person's rows to any caller "+
				"holding education.read anywhere, which is what all nine of these endpoints did from M20 "+
				"until M58 ticket 7 (D-PersonReadScope)", name)
		}
	}
}

// TestEveryPersonBindingReadIsListed closes the hole the list above leaves: a NEW
// `GET /persons/{personId}/…` handler would simply not be in it, and the guard would stay green while
// the tenth endpoint shipped ungated. The contract is the source of truth for what those endpoints
// ARE, so the list is held against the Conjure YAML rather than against itself.
//
// This is the check that would have caught the original defect. The nine handlers were all written
// before the rule had an implementation to reach for; nothing about any one of them looked wrong.
func TestEveryPersonBindingReadIsListed(t *testing.T) {
	listed := map[string]bool{}
	for _, n := range holderScopedHandlers {
		listed[n] = true
	}
	found := personBindingReadOps(t)
	if len(found) < len(holderScopedHandlers) {
		t.Fatalf("parsed only %d person-binding GET operations from the contract, but %d are listed — "+
			"the parse is broken and this check is vacuous", len(found), len(holderScopedHandlers))
	}
	for _, op := range found {
		// The Go method name is the operation name with an upper-case first letter.
		method := strings.ToUpper(op[:1]) + op[1:]
		if !listed[method] {
			t.Errorf("%s serves GET /persons/{personId}/… and is not in holderScopedHandlers — a read of "+
				"one person's rows must pass the holder read scope (D-PersonReadScope) or say in this "+
				"file why it does not", method)
		}
	}
}

// TestHolderReadableOwnsTheProjection keeps the guard above meaningful. It is a presence check on a
// helper NAME, so it is worth something only while that helper really is the single implementation:
// a handler calling ReadablePerson directly would satisfy nothing above and drift on its own, and —
// worse — would skip the instance-admin arm that lives inside the helper.
func TestHolderReadableOwnsTheProjection(t *testing.T) {
	fset := token.NewFileSet()
	pkg, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse transport package: %v", err)
	}
	for _, p := range pkg {
		for name, f := range p.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Name.Name == "holderReadable" {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					sel, sok := n.(*ast.SelectorExpr)
					if !sok || sel.Sel.Name != "ReadablePerson" {
						return true
					}
					t.Errorf("%s (%s) calls ReadablePerson outside holderReadable — it has written its own "+
						"copy of the projection and will not carry the instance-admin arm the helper "+
						"applies before it", fn.Name.Name, name)
					return true
				})
			}
		}
	}
}

// personBindingReadOps returns the operation names of every `GET /persons/{personId}/…` endpoint in
// the two education contracts. Both the block form (education.conjure.yml) and the one-line form
// (education_reference.conjure.yml) are recognised, because the two files are written differently and
// a parser that saw only one would silently miss six endpoints.
func personBindingReadOps(t *testing.T) []string {
	t.Helper()
	var (
		// one-line: `listGrantHoldings: { http: "GET /persons/{personId}/grant-holdings", …`
		inline = regexp.MustCompile(`^\s*([a-z][A-Za-z0-9]*):\s*\{\s*http:\s*"GET /persons/\{personId\}/`)
		// block: `listEnrollments:` … `http: GET /persons/{personId}/enrollments`
		opLine   = regexp.MustCompile(`^\s{6}([a-z][A-Za-z0-9]*):\s*$`)
		httpLine = regexp.MustCompile(`^\s*http:\s*"?GET /persons/\{personId\}/`)
	)
	var out []string
	for _, path := range []string{
		"../../../api/education.conjure.yml",
		"../../../api/education_reference.conjure.yml",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(raw)
		var pending string
		for _, line := range strings.Split(body, "\n") {
			if m := inline.FindStringSubmatch(line); m != nil {
				out = append(out, m[1])
				pending = ""
				continue
			}
			if m := opLine.FindStringSubmatch(line); m != nil {
				pending = m[1]
				continue
			}
			if pending != "" && httpLine.MatchString(line) {
				out = append(out, pending)
				pending = ""
			}
		}
	}
	return out
}
