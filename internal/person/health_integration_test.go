// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the M36 health & vulnerability records (D-HealthVulnerability) against a real
// Postgres. Proves the exit criteria:
//
//   - a category-level health record is envelope-encrypted at rest (the detail ciphertext holds NO
//     plaintext + the blind index is populated), decrypts to the same detail, is single-active-per-(person,
//     kind) (a second upsert of the same kind replaces in place), and requires legalBasis (Art. 9);
//
//   - an insurance row (pii:sensitive, plaintext) round-trips;
//
//   - purge CRYPTO-ERASES the health records (envelope dropped, row tombstone) and HARD-DELETES insurance.
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

func TestPersonHealth(t *testing.T) {
	ctx := context.Background()
	svc, _, sens, pool := newServices(t, 721)
	p := newPerson(t, svc, "Olha Zdorovenko")

	// ---- health record: legalBasis required + invalid kind rejected ----
	if _, err := sens.UpsertHealthRecord(ctx, domain.HealthRecord{
		PersonID: p.ID, Kind: "disability", Detail: "wheelchair user", // no legalBasis
	}); err == nil {
		t.Fatal("expected health record without legalBasis to be rejected")
	}
	if _, err := sens.UpsertHealthRecord(ctx, domain.HealthRecord{
		PersonID: p.ID, Kind: "not_a_kind", Detail: "x", LegalBasis: "explicit_consent",
	}); err == nil {
		t.Fatal("expected invalid health kind to be rejected")
	}
	if _, err := sens.UpsertHealthRecord(ctx, domain.HealthRecord{
		PersonID: p.ID, Kind: "disability", Detail: "wheelchair user", LegalBasis: "no_such_basis",
	}); err == nil {
		t.Fatal("expected unknown legalBasis code to be rejected (FK)")
	}

	// ---- health record: round-trip + encryption at rest ----
	const detail = "mobility disability — wheelchair"
	h, err := sens.UpsertHealthRecord(ctx, domain.HealthRecord{
		PersonID: p.ID, Kind: "disability", Detail: detail, IsPublicRecord: true,
		LegalBasis: "explicit_consent", Source: "operator_verified", Confidence: "confirmed", AssessedAt: "2024-03-01",
	})
	if err != nil {
		t.Fatalf("add health record: %v", err)
	}
	if h.Detail != detail || h.Kind != "disability" || !h.IsPublicRecord {
		t.Fatalf("health record round-trip mismatch: %+v", h)
	}

	// Ciphertext at rest holds NO plaintext detail, and the blind index is populated.
	var ct, bi []byte
	if err := pool.QueryRow(ctx,
		`SELECT detail_ciphertext, detail_blind_index FROM oikumenea.person_health_records WHERE id = $1`, h.ID).
		Scan(&ct, &bi); err != nil {
		t.Fatalf("read health envelope: %v", err)
	}
	if len(ct) == 0 || bytes.Contains(ct, []byte("wheelchair")) {
		t.Fatalf("health ciphertext leaks plaintext or is empty (len=%d)", len(ct))
	}
	if len(bi) == 0 {
		t.Fatal("health blind index not populated")
	}

	// Decrypt round-trip via the read path.
	list, err := sens.ListHealthRecords(ctx, p.ID)
	if err != nil || len(list) != 1 || list[0].Detail != detail {
		t.Fatalf("health decrypt round-trip mismatch: %+v err=%v", list, err)
	}

	// Single active per (person, kind): a second upsert of the same kind replaces in place (same id).
	h2, err := sens.UpsertHealthRecord(ctx, domain.HealthRecord{
		PersonID: p.ID, Kind: "disability", Detail: "updated category note", LegalBasis: "explicit_consent",
	})
	if err != nil {
		t.Fatalf("re-upsert health record: %v", err)
	}
	if h2.ID != h.ID {
		t.Fatalf("health record not single-active-per-(person,kind): %s != %s", h2.ID, h.ID)
	}
	// A different kind is a distinct active row.
	if _, err := sens.UpsertHealthRecord(ctx, domain.HealthRecord{
		PersonID: p.ID, Kind: "hospitalization", Detail: "2019 inpatient stay", LegalBasis: "explicit_consent",
	}); err != nil {
		t.Fatalf("add second-kind health record: %v", err)
	}
	if list, err = sens.ListHealthRecords(ctx, p.ID); err != nil || len(list) != 2 {
		t.Fatalf("expected 2 active health records (one per kind): %+v err=%v", list, err)
	}

	// ---- insurance: round-trip ----
	ins, err := sens.UpsertInsurance(ctx, domain.Insurance{
		PersonID: p.ID, Type: "health", Provider: "Oranta", EmployerSponsored: true,
		ValidFrom: "2023-01-01", ValidTo: "2025-01-01", Source: "self_declared", Confidence: "confirmed",
	})
	if err != nil {
		t.Fatalf("add insurance: %v", err)
	}
	if ins.Type != "health" || ins.Provider != "Oranta" || !ins.EmployerSponsored {
		t.Fatalf("insurance round-trip mismatch: %+v", ins)
	}
	if _, err := sens.UpsertInsurance(ctx, domain.Insurance{PersonID: p.ID, Type: "bogus"}); err == nil {
		t.Fatal("expected invalid insurance type to be rejected")
	}

	// ---- purge: crypto-erase health, hard-delete insurance ----
	svcNow, _, _, _ := newServices(t, 0) // zero-grace purge; auto-wires the PersonPurged bus (R-09)
	if _, err := svcNow.DeactivatePerson(ctx, p.ID, "test"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcNow.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var healthRows, healthEnvelopes, insuranceRows int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM oikumenea.person_health_records WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_health_records WHERE person_id=$1 AND detail_ciphertext IS NOT NULL),
		(SELECT count(*) FROM oikumenea.person_insurance WHERE person_id=$1)`, p.ID).
		Scan(&healthRows, &healthEnvelopes, &insuranceRows); err != nil {
		t.Fatalf("post-purge counts: %v", err)
	}
	if healthRows != 2 || healthEnvelopes != 0 {
		t.Fatalf("health records should survive as crypto-erased tombstones: rows=%d envelopes=%d", healthRows, healthEnvelopes)
	}
	if insuranceRows != 0 {
		t.Fatalf("insurance (pii:sensitive) should be hard-deleted on purge: rows=%d", insuranceRows)
	}
}
