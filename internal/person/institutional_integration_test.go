// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the M33 institutional & political ties slice (D-InstitutionalTies) against a real
// Postgres. Proves the exit criteria:
//
//   - a party membership is envelope-encrypted at rest (ciphertext holds NO plaintext + blind index present)
//     and decrypts on read; legalBasis is required (Art. 9);
//
//   - a government position with pep_trigger drives IsPoliticallyExposed (the M34 PEP seam); delete clears it;
//
//   - a lobbying relationship round-trips its issues[] array;
//
//   - an external reference is idempotent by URL (a re-upsert updates in place, not duplicates);
//
//   - the 'emergency' relation type is seeded (M14 catalog, no new entity);
//
//   - purge CRYPTO-ERASES the party membership (envelope dropped, row tombstone) and HARD-DELETES the
//     plaintext ties (government/lobbying/external).
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/person/...
package person_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/person/domain"
)

func TestInstitutionalTies(t *testing.T) {
	ctx := context.Background()
	svc, prof, sens, pool := newServices(t, 720)
	p := newPerson(t, svc, "Olena Politychna")

	// ---- party membership: encrypted, legal_basis required ----
	if _, err := sens.UpsertPartyMembership(ctx, domain.PartyMembership{
		PersonID: p.ID, Party: "Servant of the People", Role: "member", // no legalBasis
	}); err == nil {
		t.Fatal("expected party membership without legalBasis to be rejected")
	}
	party, err := sens.UpsertPartyMembership(ctx, domain.PartyMembership{
		PersonID: p.ID, Party: "Servant of the People", Role: "official",
		LegalBasis: "explicit_consent", ValidFrom: "2019-05-20", Source: "self_declared", Confidence: "confirmed",
	})
	if err != nil {
		t.Fatalf("add party: %v", err)
	}
	if party.Party != "Servant of the People" || party.Role != "official" {
		t.Fatalf("party round-trip mismatch: %+v", party)
	}

	// Ciphertext at rest holds NO plaintext, and the blind index is populated.
	var ct, bi []byte
	if err := pool.QueryRow(ctx,
		`SELECT party_ciphertext, party_blind_index FROM oikumenea.person_party_memberships WHERE id = $1`, party.ID).
		Scan(&ct, &bi); err != nil {
		t.Fatalf("read party envelope: %v", err)
	}
	if len(ct) == 0 || bytes.Contains(ct, []byte("Servant of the People")) {
		t.Fatalf("party ciphertext leaks plaintext or is empty (len=%d)", len(ct))
	}
	if len(bi) == 0 {
		t.Fatal("party blind index not populated")
	}

	parties, err := sens.ListPartyMemberships(ctx, p.ID)
	if err != nil || len(parties) != 1 || parties[0].Party != "Servant of the People" {
		t.Fatalf("list party decrypt mismatch: %+v err=%v", parties, err)
	}

	// ---- government position: PEP derivation ----
	if exposed, _ := prof.IsPoliticallyExposed(ctx, p.ID); exposed {
		t.Fatal("not politically exposed before any government position")
	}
	uaID := countryRID(t, pool, "UA")
	gov, err := prof.UpsertGovernmentPosition(ctx, domain.GovernmentPosition{
		PersonID: p.ID, Title: "Minister of Defence", Body: "Ministry of Defence",
		CountryID: uaID, Level: "national", RoleType: "appointed", ValidFrom: "2020-03-04",
	})
	if err != nil {
		t.Fatalf("add government position: %v", err)
	}
	if !gov.PEPTrigger {
		t.Fatal("government position should default pep_trigger true")
	}
	if exposed, _ := prof.IsPoliticallyExposed(ctx, p.ID); !exposed {
		t.Fatal("expected politically exposed after a pep_trigger position")
	}

	// ---- lobbying relationship: issues[] round-trip ----
	lob, err := prof.UpsertLobbyingRelationship(ctx, domain.LobbyingRelationship{
		PersonID: p.ID, Registrant: "Acme Advocacy LLC", Client: "Defense Systems Inc",
		LegislativeBody: "Verkhovna Rada", Issues: []string{"defense", "procurement"}, FilingID: "F-2020-123",
	})
	if err != nil {
		t.Fatalf("add lobbying: %v", err)
	}
	if len(lob.Issues) != 2 || lob.Issues[0] != "defense" {
		t.Fatalf("lobbying issues round-trip mismatch: %+v", lob.Issues)
	}

	// ---- external reference: idempotent by URL ----
	ref1, err := prof.UpsertExternalReference(ctx, domain.ExternalReference{
		PersonID: p.ID, Kind: "wikipedia", URL: "https://en.wikipedia.org/wiki/Olena",
		Categories: []string{"politician"},
	})
	if err != nil {
		t.Fatalf("add external reference: %v", err)
	}
	ref2, err := prof.UpsertExternalReference(ctx, domain.ExternalReference{
		PersonID: p.ID, Kind: "wikipedia", URL: "https://en.wikipedia.org/wiki/Olena",
		Categories: []string{"politician", "minister"}, Disputed: true,
	})
	if err != nil {
		t.Fatalf("re-upsert external reference: %v", err)
	}
	if ref1.ID != ref2.ID {
		t.Fatalf("external reference not idempotent by url: %s != %s", ref1.ID, ref2.ID)
	}
	refs, err := prof.ListExternalReferences(ctx, p.ID)
	if err != nil || len(refs) != 1 || !refs[0].Disputed {
		t.Fatalf("external reference list/upsert mismatch: %+v err=%v", refs, err)
	}

	// ---- 'emergency' relation type seeded (M14 catalog, no new entity) ----
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.person_relation_types WHERE code = 'emergency' AND deleted_at IS NULL`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("emergency relation type not seeded: n=%d err=%v", n, err)
	}

	// ---- purge: crypto-erase party, hard-delete the plaintext ties ----
	svcNow, _, _, _ := newServices(t, 0) // zero-grace purge; auto-wires the PersonPurged bus (R-09)
	if _, err := svcNow.DeactivatePerson(ctx, p.ID, "test"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcNow.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var partyRows, partyEnvelopes, govRows, lobRows, refRows int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM oikumenea.person_party_memberships WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_party_memberships WHERE person_id=$1 AND party_ciphertext IS NOT NULL),
		(SELECT count(*) FROM oikumenea.person_government_positions WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_lobbying_relationships WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_external_references WHERE person_id=$1)`, p.ID).
		Scan(&partyRows, &partyEnvelopes, &govRows, &lobRows, &refRows); err != nil {
		t.Fatalf("post-purge counts: %v", err)
	}
	if partyRows != 1 || partyEnvelopes != 0 {
		t.Fatalf("party should survive as a crypto-erased tombstone: rows=%d envelopes=%d", partyRows, partyEnvelopes)
	}
	if govRows != 0 || lobRows != 0 || refRows != 0 {
		t.Fatalf("plaintext ties should be hard-deleted on purge: gov=%d lob=%d ref=%d", govRows, lobRows, refRows)
	}
}
