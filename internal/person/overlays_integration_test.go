// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the M35 financial/behavioural/psychological overlays (D-PersonOverlays) against a
// real Postgres. Proves the exit criteria:
//
//   - a crypto wallet round-trips (address/chain/balance) and dedups an active (person,chain,address);
//
//   - a personality profile is declared/assessment-only — a method outside {self_declared_survey,
//     hr_assessment} is rejected, and a valid one round-trips;
//
//   - the inferred political leaning is envelope-encrypted at rest (ciphertext holds NO plaintext spectrum
//
//   - blind index present), decrypts to the same spectrum, is single-active-per-person (a second set
//     replaces in place), and requires legalBasis (Art. 9);
//
//   - purge HARD-DELETES the wallet + personality (pii:sensitive) and CRYPTO-ERASES the leaning (envelope
//     dropped, row tombstone).
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

func TestPersonOverlays(t *testing.T) {
	ctx := context.Background()
	svc, _, sens, pool := newServices(t, 720)
	p := newPerson(t, svc, "Ihor Overlaienko")

	// ---- crypto wallet: round-trip + dedup ----
	bal := 12345.67
	w, err := sens.UpsertCryptoWallet(ctx, domain.CryptoWallet{
		PersonID: p.ID, Address: "0xAbC123", Chain: "ethereum",
		AttributionMethod: "blockchain_analysis", BalanceUSDApprox: &bal,
		FirstSeen: "2021-01-01", Source: "imported", Confidence: "probable",
	})
	if err != nil {
		t.Fatalf("add crypto wallet: %v", err)
	}
	if w.Chain != "ethereum" || w.BalanceUSDApprox == nil || *w.BalanceUSDApprox != bal {
		t.Fatalf("crypto wallet round-trip mismatch: %+v", w)
	}
	if _, err := sens.UpsertCryptoWallet(ctx, domain.CryptoWallet{
		PersonID: p.ID, Address: "0xAbC123", Chain: "ethereum", AttributionMethod: "leak",
	}); err == nil {
		t.Fatal("expected duplicate active (person,chain,address) crypto wallet to conflict")
	}
	wallets, err := sens.ListCryptoWallets(ctx, p.ID)
	if err != nil || len(wallets) != 1 {
		t.Fatalf("crypto wallet list mismatch: %+v err=%v", wallets, err)
	}

	// ---- personality: declared/assessment-only + round-trip ----
	if _, err := sens.UpsertPersonality(ctx, domain.Personality{
		PersonID: p.ID, Framework: "mbti", Result: "INTJ", Method: "text_inference",
	}); err == nil {
		t.Fatal("expected inferred personality method to be rejected (declared/assessment only)")
	}
	per, err := sens.UpsertPersonality(ctx, domain.Personality{
		PersonID: p.ID, Framework: "mbti", Result: "INTJ",
		Instrument: "16Personalities", Method: "self_declared_survey", AssessedAt: "2023-06-01",
	})
	if err != nil {
		t.Fatalf("add personality: %v", err)
	}
	if per.Result != "INTJ" || per.Method != "self_declared_survey" {
		t.Fatalf("personality round-trip mismatch: %+v", per)
	}

	// ---- political leaning: encrypted, single-active, legalBasis required ----
	if _, err := sens.SetPoliticalLeaning(ctx, domain.PoliticalLeaning{
		PersonID: p.ID, Spectrum: -0.4, // no legalBasis
	}); err == nil {
		t.Fatal("expected political leaning without legalBasis to be rejected")
	}
	if _, err := sens.SetPoliticalLeaning(ctx, domain.PoliticalLeaning{
		PersonID: p.ID, Spectrum: 2.0, LegalBasis: "legitimate_interest", // out of [-1,1]
	}); err == nil {
		t.Fatal("expected out-of-range spectrum to be rejected")
	}
	lean, err := sens.SetPoliticalLeaning(ctx, domain.PoliticalLeaning{
		PersonID: p.ID, Spectrum: -0.42, InferenceSources: []string{"social_media", "voting_record"},
		LegalBasis: "legitimate_interest", Confidence: "possible", AssessedAt: "2024-02-15",
	})
	if err != nil {
		t.Fatalf("set political leaning: %v", err)
	}
	if lean.Spectrum != -0.42 || len(lean.InferenceSources) != 2 {
		t.Fatalf("political leaning round-trip mismatch: %+v", lean)
	}

	// Ciphertext at rest holds NO plaintext spectrum, and the blind index is populated.
	var ct, bi []byte
	if err := pool.QueryRow(ctx,
		`SELECT leaning_ciphertext, leaning_blind_index FROM oikumenea.person_political_leaning WHERE id = $1`, lean.ID).
		Scan(&ct, &bi); err != nil {
		t.Fatalf("read leaning envelope: %v", err)
	}
	if len(ct) == 0 || bytes.Contains(ct, []byte("-0.42")) {
		t.Fatalf("leaning ciphertext leaks plaintext or is empty (len=%d)", len(ct))
	}
	if len(bi) == 0 {
		t.Fatal("leaning blind index not populated")
	}

	// Single active per person: a second Set replaces in place (same row id).
	lean2, err := sens.SetPoliticalLeaning(ctx, domain.PoliticalLeaning{
		PersonID: p.ID, Spectrum: 0.15, LegalBasis: "legitimate_interest",
	})
	if err != nil {
		t.Fatalf("re-set political leaning: %v", err)
	}
	if lean2.ID != lean.ID {
		t.Fatalf("political leaning not single-active-per-person: %s != %s", lean2.ID, lean.ID)
	}
	got, err := sens.GetPoliticalLeaning(ctx, p.ID)
	if err != nil || got.Spectrum != 0.15 {
		t.Fatalf("political leaning get after replace mismatch: %+v err=%v", got, err)
	}

	// ---- purge: hard-delete wallet+personality, crypto-erase leaning ----
	svcNow, _, _, _ := newServices(t, 0) // zero-grace purge; auto-wires the PersonPurged bus (R-09)
	if _, err := svcNow.DeactivatePerson(ctx, p.ID, "test"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcNow.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var walletRows, personalityRows, leaningRows, leaningEnvelopes int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM oikumenea.person_crypto_wallets WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_personality WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_political_leaning WHERE person_id=$1),
		(SELECT count(*) FROM oikumenea.person_political_leaning WHERE person_id=$1 AND leaning_ciphertext IS NOT NULL)`, p.ID).
		Scan(&walletRows, &personalityRows, &leaningRows, &leaningEnvelopes); err != nil {
		t.Fatalf("post-purge counts: %v", err)
	}
	if walletRows != 0 || personalityRows != 0 {
		t.Fatalf("pii:sensitive overlays should be hard-deleted on purge: wallets=%d personality=%d", walletRows, personalityRows)
	}
	if leaningRows != 1 || leaningEnvelopes != 0 {
		t.Fatalf("leaning should survive as a crypto-erased tombstone: rows=%d envelopes=%d", leaningRows, leaningEnvelopes)
	}
}
