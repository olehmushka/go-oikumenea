// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the M38 criminal / arrest / court records (D-LegalRecords) against a real
// Postgres. Proves the exit criteria:
//
//   - a legal record requires legalBasis (Art. 10) and a valid kind + disposition (arrest ≠ guilt),
//     and rejects a suppressed_reason without is_suppressed (and vice versa);
//
//   - the category-level offence detail is envelope-encrypted at rest (ciphertext holds NO plaintext +
//     the blind index is populated) and decrypts to the same detail; jurisdiction resolves to its
//     geo_countries RID and reads back as the ISO code; many records per person are allowed;
//
//   - a sealed/expunged (suppressed) record is WITHHELD from the normal read (includeSuppressed=false)
//     and revealed when includeSuppressed=true;
//
//   - purge CRYPTO-ERASES the records (envelope dropped, row tombstone survives).
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/person/...
package person_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

func TestPersonLegalRecords(t *testing.T) {
	ctx := context.Background()
	svc, _, sens, pool := newServices(t, 738)
	p := newPerson(t, svc, "Ivan Sudovyi")

	// ---- validation: legalBasis + kind + disposition + suppression invariant ----
	if _, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "arrest", Disposition: "pending", Detail: "x", // no legalBasis
	}); err == nil {
		t.Fatal("expected legal record without legalBasis to be rejected")
	}
	if _, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "not_a_kind", Disposition: "pending", Detail: "x", LegalBasis: "legal_obligation",
	}); err == nil {
		t.Fatal("expected invalid kind to be rejected")
	}
	if _, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "arrest", Disposition: "guilty_maybe", Detail: "x", LegalBasis: "legal_obligation",
	}); err == nil {
		t.Fatal("expected invalid disposition to be rejected")
	}
	if _, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "arrest", Disposition: "pending", Detail: "x", LegalBasis: "legal_obligation",
		SuppressedReason: "sealed", // reason set but not IsSuppressed
	}); err == nil {
		t.Fatal("expected suppressed_reason without is_suppressed to be rejected")
	}
	if _, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "arrest", Disposition: "pending", Detail: "x", LegalBasis: "legal_obligation",
		Jurisdiction: "ZZ", // unknown country
	}); err == nil {
		t.Fatal("expected unknown jurisdiction code to be rejected (FK)")
	}

	// ---- round-trip + encryption at rest + jurisdiction resolution ----
	const detail = "theft (category-level)"
	r, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "criminal_conviction", Disposition: "convicted", Detail: detail,
		Jurisdiction: "UA", OccurredAt: "2020-05-01", DispositionDate: "2021-02-01",
		LegalBasis: "legal_obligation", Source: "operator_verified", Confidence: "confirmed",
	})
	if err != nil {
		t.Fatalf("add legal record: %v", err)
	}
	if r.Detail != detail || r.Kind != "criminal_conviction" || r.Disposition != "convicted" || r.Jurisdiction != "UA" {
		t.Fatalf("legal record round-trip mismatch: %+v", r)
	}

	// Ciphertext at rest holds NO plaintext detail, and the blind index is populated.
	var ct, bi []byte
	if err := pool.QueryRow(ctx,
		`SELECT detail_ciphertext, detail_blind_index FROM oikumenea.person_legal_records WHERE id = $1`, r.ID).
		Scan(&ct, &bi); err != nil {
		t.Fatalf("read legal envelope: %v", err)
	}
	if len(ct) == 0 || bytes.Contains(ct, []byte("theft")) {
		t.Fatalf("legal ciphertext leaks plaintext or is empty (len=%d)", len(ct))
	}
	if len(bi) == 0 {
		t.Fatal("legal blind index not populated")
	}

	// Decrypt round-trip + jurisdiction code read back via the list join.
	list, err := sens.ListLegalRecords(ctx, p.ID, false)
	if err != nil || len(list) != 1 || list[0].Detail != detail || list[0].Jurisdiction != "UA" {
		t.Fatalf("legal decrypt round-trip mismatch: %+v err=%v", list, err)
	}

	// Many records per person (no one-active-per-kind): add an arrest too.
	if _, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "arrest", Disposition: "dismissed", Detail: "public order (category-level)",
		LegalBasis: "legal_obligation",
	}); err != nil {
		t.Fatalf("add arrest record: %v", err)
	}
	if list, err = sens.ListLegalRecords(ctx, p.ID, false); err != nil || len(list) != 2 {
		t.Fatalf("expected 2 active legal records: %+v err=%v", list, err)
	}

	// ---- suppression: a sealed record is withheld unless includeSuppressed ----
	sealed, err := sens.UpsertLegalRecord(ctx, domain.LegalRecord{
		PersonID: p.ID, Kind: "court_judgment", Disposition: "sealed", Detail: "juvenile matter (category-level)",
		IsSuppressed: true, SuppressedReason: "sealed", LegalBasis: "legal_obligation",
	})
	if err != nil {
		t.Fatalf("add sealed record: %v", err)
	}
	visible, err := sens.ListLegalRecords(ctx, p.ID, false)
	if err != nil {
		t.Fatalf("list (no suppressed): %v", err)
	}
	for _, x := range visible {
		if x.ID == sealed.ID {
			t.Fatal("sealed record must be withheld from the normal read gate")
		}
	}
	if len(visible) != 2 {
		t.Fatalf("normal read should show 2 non-suppressed records, got %d", len(visible))
	}
	all, err := sens.ListLegalRecords(ctx, p.ID, true)
	if err != nil || len(all) != 3 {
		t.Fatalf("read-suppressed should reveal all 3 records: %+v err=%v", all, err)
	}

	// ---- purge: crypto-erase all legal records (row tombstones survive) ----
	svcNow, _, _, _ := newServices(t, 0) // zero-grace purge; auto-wires the PersonPurged bus (R-09)
	if _, err := svcNow.DeactivatePerson(ctx, p.ID, "test"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcNow.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var rows, envelopes int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM oikumenea.person_legal_records WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_legal_records WHERE person_id=$1 AND detail_ciphertext IS NOT NULL)`, p.ID).
		Scan(&rows, &envelopes); err != nil {
		t.Fatalf("post-purge counts: %v", err)
	}
	if rows != 3 || envelopes != 0 {
		t.Fatalf("legal records should survive as crypto-erased tombstones: rows=%d envelopes=%d", rows, envelopes)
	}
}
