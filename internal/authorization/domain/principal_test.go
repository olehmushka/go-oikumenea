// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"testing"
)

// TestPrincipalGrantSatisfies pins the org-scoping semantics that bound a connector's blast radius
// (M51 / D-ServiceIdentities). The asymmetry in the last case is the point of the design: a grant
// confined to one organization must NOT satisfy a request that names no organization, because such an
// endpoint is not org-qualified and could reach data outside that org.
func TestPrincipalGrantSatisfies(t *testing.T) {
	const (
		orgA = "org-a"
		orgB = "org-b"
	)
	imports := Permission("import.manage")
	other := Permission("person.read")

	instanceWide := PrincipalGrant{Permission: imports}             // org_id NULL
	confinedToA := PrincipalGrant{Permission: imports, OrgID: orgA} // org_id = org-a

	cases := []struct {
		name   string
		grant  PrincipalGrant
		action Permission
		orgID  string
		want   bool
		why    string
	}{
		{"instance-wide answers an unqualified request", instanceWide, imports, "", true,
			"the whole point of an instance-wide grant"},
		{"instance-wide answers any org", instanceWide, imports, orgA, true,
			"instance-wide is a superset of every org"},
		{"org-confined answers its own org", confinedToA, imports, orgA, true,
			"the connector's own organization"},
		{"org-confined REJECTS another org", confinedToA, imports, orgB, false,
			"a church scraper must not rewrite another organization's data"},
		{"org-confined REJECTS an unqualified request", confinedToA, imports, "", false,
			"an unqualified endpoint could reach outside the confined org"},
		{"wrong action never matches", instanceWide, other, "", false,
			"a grant answers exactly one permission code"},
		{"wrong action never matches even in-org", confinedToA, other, orgA, false,
			"the code is checked before the scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.grant.Satisfies(tc.action, tc.orgID); got != tc.want {
				t.Errorf("Satisfies(%q, orgID=%q) = %v, want %v — %s", tc.action, tc.orgID, got, tc.want, tc.why)
			}
		})
	}
}

// TestPrincipalGrantInputValidate: a grant must name a principal and a code from the CLOSED catalog.
// An unknown code would otherwise sit in the table forever, matching nothing and reading as authority.
func TestPrincipalGrantInputValidate(t *testing.T) {
	valid := PrincipalGrantInput{PrincipalID: "p-1", Permission: PermImportManage}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if err := (PrincipalGrantInput{Permission: PermImportManage}).Validate(); err == nil {
		t.Error("accepted a grant with no principal")
	}
	if err := (PrincipalGrantInput{PrincipalID: "p-1", Permission: "not.a.real.code"}).Validate(); err == nil {
		t.Error("accepted an unknown permission code; the catalog is closed (D-Code)")
	}
}

// TestServicePrincipalPermissionsAreInstanceScope: minting or re-granting a machine identity is an
// instance-admin act. If these codes ever became unit-scoped, a unit-level role could hand out
// machine credentials — so both the catalog membership and the instance-scope classification are
// pinned here.
func TestServicePrincipalPermissionsAreInstanceScope(t *testing.T) {
	for _, p := range []Permission{PermServicePrincipalRead, PermServicePrincipalManage} {
		if !IsKnownPermission(string(p)) {
			t.Errorf("%s missing from the closed catalog — writes naming it would be rejected", p)
		}
		if !IsInstanceScope(string(p)) {
			t.Errorf("%s is not instance-scope; a unit role could then grant machine credentials", p)
		}
	}
}

// TestServicePrincipalPermissionsAbsentFromBaseRoles: instance-scope codes are satisfiable ONLY via
// the instance-admin plane (see pdp.go), so putting one in a base role is dead weight that reads as a
// grant. This guards against a future "just add it to admin" edit.
func TestServicePrincipalPermissionsAbsentFromBaseRoles(t *testing.T) {
	for _, br := range BaseRoles() {
		for _, p := range br.Permissions {
			if p == PermServicePrincipalRead || p == PermServicePrincipalManage {
				t.Errorf("base role %q carries instance-scope %s; the PDP can never satisfy it from a role", br.Code, p)
			}
		}
	}
}
