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
		PermPersonHealthRead,
		PermPersonLegalRecordRead, // D-LegalRecords, M38 (base read; read-suppressed is stricter, checked below)
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

	// (d) person.legal-record.read-suppressed is the strictest gate (D-LegalRecords, M38): in the closed
	// catalog + unit-scope, but reachable through NO base role at all — not even sensitive-reader — so
	// sealed/expunged records require an explicit, separately granted capability.
	suppressed := PermPersonLegalRecordReadSuppressed
	if !IsKnownPermission(string(suppressed)) {
		t.Fatalf("%q must be in the closed permission catalog", suppressed)
	}
	if IsInstanceScope(string(suppressed)) {
		t.Fatalf("%q must be unit-scope (assignable), not instance-plane", suppressed)
	}
	for _, br := range BaseRoles() {
		if roles[br.Code][suppressed] {
			t.Fatalf("base role %q must NOT grant %q (the strictest gate is granted explicitly only)", br.Code, suppressed)
		}
	}
}

// TestRelationshipReadsGatedByOwnCodes is the D-LinkPermissions analog of the D-DataScope rule above:
// the person relationship graph (who someone is partnered with / related to / vouches for / lists as
// next of kin / associates with, and where they live) carries per-link read codes that (a) are in the
// closed catalog and unit-scope, (b) are reachable through NO graduated base role — person.read alone
// no longer discloses the personal graph — and (c) are bundled by exactly the standalone
// person-relationship-reader role. The SAME codes gate the person module's dedicated list endpoints and
// the link-traversal arms (cmd/oikumenea/link_descriptors.go), so the page and the object graph agree.
func TestRelationshipReadsGatedByOwnCodes(t *testing.T) {
	rels := []Permission{
		PermPersonPartnershipRead, PermPersonKinshipRead, PermPersonGuardianshipRead,
		PermPersonSponsorshipRead, PermPersonNextOfKinRead, PermPersonAssociationRead,
		PermPersonAddressRead,
	}

	for _, p := range rels {
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

	// No graduated role — nor auditor/sensitive-reader — grants a relationship read: it is additive.
	for _, code := range []string{
		BaseRoleUnitReader, BaseRoleUnitManager, BaseRoleUnitAdmin, BaseRoleAuditor, BaseRoleSensitiveReader,
	} {
		set, ok := roles[code]
		if !ok {
			t.Fatalf("base role %q not seeded", code)
		}
		for _, p := range rels {
			if set[p] {
				t.Fatalf("base role %q must NOT grant the relationship read %q (D-LinkPermissions)", code, p)
			}
		}
	}

	// person-relationship-reader grants exactly the relationship set — no more, no less.
	rr, ok := roles[BaseRolePersonRelationshipReader]
	if !ok {
		t.Fatalf("person-relationship-reader base role must be seeded")
	}
	if len(rr) != len(rels) {
		t.Fatalf("person-relationship-reader must hold exactly %d permissions, got %d", len(rels), len(rr))
	}
	for _, p := range rels {
		if !rr[p] {
			t.Fatalf("person-relationship-reader must grant %q", p)
		}
	}
}
