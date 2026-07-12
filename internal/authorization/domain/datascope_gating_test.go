package domain

import "testing"

// TestSensitivePersonReadsGatedByOwnCodes proves the D-DataScope aggregation rule (review R-14) at the
// base-role level: the three pii:special Art.9 person reads are (a) in the closed catalog, (b) reachable
// through NO graduated base role — not even unit-admin — and (c) bundled by exactly the standalone
// sensitive-reader role. So no single graduated grant unlocks the ethnicity + politics + party
// aggregation; reading Art.9 person data requires the explicit sensitive-reader grant. The transport
// gates (ListEthnicities / GetPoliticalLeaning / ListPartyMemberships) enforce these same codes.
func TestSensitivePersonReadsGatedByOwnCodes(t *testing.T) {
	special := []Permission{
		PermPersonEthnicityRead,
		PermPersonPoliticalLeaningRead,
		PermPersonPartyMembershipRead,
	}

	// (a) each is a known, unit-scope permission.
	for _, p := range special {
		if !IsKnownPermission(string(p)) {
			t.Fatalf("%q must be in the closed permission catalog", p)
		}
		if IsInstanceScope(string(p)) {
			t.Fatalf("%q must be unit-scope (assignable), not instance-plane", p)
		}
	}

	roles := map[string]map[Permission]bool{}
	for _, br := range BaseRoles() {
		set := map[Permission]bool{}
		for _, p := range br.Permissions {
			set[p] = true
		}
		roles[br.Code] = set
	}

	// (b) no graduated role (reader → manager → admin) nor auditor grants any special read.
	for _, code := range []string{BaseRoleUnitReader, BaseRoleUnitManager, BaseRoleUnitAdmin, BaseRoleAuditor} {
		set, ok := roles[code]
		if !ok {
			t.Fatalf("base role %q not seeded", code)
		}
		for _, p := range special {
			if set[p] {
				t.Fatalf("base role %q must NOT grant the pii:special read %q (D-DataScope aggregation rule)", code, p)
			}
		}
	}
	// person.read must still ride the base reader (the surrounding directory surface is unchanged).
	if !roles[BaseRoleUnitReader][PermPersonRead] {
		t.Fatalf("unit-reader must still hold person.read")
	}

	// (c) sensitive-reader grants exactly the three special reads — no more, no less.
	sr, ok := roles[BaseRoleSensitiveReader]
	if !ok {
		t.Fatalf("sensitive-reader base role must be seeded")
	}
	if len(sr) != len(special) {
		t.Fatalf("sensitive-reader must hold exactly %d permissions, got %d", len(special), len(sr))
	}
	for _, p := range special {
		if !sr[p] {
			t.Fatalf("sensitive-reader must grant %q", p)
		}
	}
}
