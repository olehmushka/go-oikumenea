// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Purge fan-out width baseline (review R-24). A person purge runs an `atomic` fan-out across many
// modules in ONE transaction (D-EventOutbox / patterns.md). Nothing measured that width, so it grew
// invisibly milestone by milestone. This test records the statement count of a representative
// person-family purge (personprofile hard-delete + personsensitive crypto-erase — where per-person row
// deletion concentrates) and asserts a GENEROUS budget: a step-change tripwire that fires only on a
// real regression, not a tight lock. The complementary automated signal — a new *module* joining the
// fan-out — is TestPersonFanoutWidthGuard (cmd/oikumenea); the cross-module width itself stays under the
// D-EventOutbox decision gate, now with a documented baseline in patterns.md.
package person_test

import (
	"context"
	"testing"
	"time"

	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// purgeWidthBudget caps the statements in one person-family purge transaction. Set to ~2× the observed
// baseline (recorded in docs/architecture/patterns.md) so it trips only on a step-change regression —
// e.g. a new atomic subscriber, or an N+1 sneaking into an erase handler. Bump it (and the patterns.md
// number) deliberately when the baseline legitimately moves.
const purgeWidthBudget = 80

func TestPurgeWidthBudget(t *testing.T) {
	ctx := context.Background()
	svc, prof, sens, pool := newServices(t, 0)
	p := newPerson(t, svc, "Purge Width Baseline")

	// Seed both erase paths the person-family fan-out covers.
	// personprofile (hard-delete on purge):
	if _, err := prof.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: countryRID(t, pool, "UA"), Basis: "birth"}); err != nil {
		t.Fatalf("citizenship: %v", err)
	}
	if _, err := prof.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "personal", Address: "width@example.com", IsPrimary: true}); err != nil {
		t.Fatalf("email: %v", err)
	}
	// personsensitive (crypto-erase / hard-delete on purge):
	ethCode := seedEthnicityType(t, pool, "Ukrainian")
	if _, err := sens.AddEthnicity(ctx, p.ID, ethCode, "explicit_consent", "self_declared", "confirmed"); err != nil {
		t.Fatalf("ethnicity: %v", err)
	}
	bal := 1000.0
	if _, err := sens.UpsertCryptoWallet(ctx, domain.CryptoWallet{
		PersonID: p.ID, Address: "0xWidth", Chain: "ethereum",
		AttributionMethod: "blockchain_analysis", BalanceUSDApprox: &bal,
	}); err != nil {
		t.Fatalf("wallet: %v", err)
	}
	if _, err := sens.UpsertPersonality(ctx, domain.Personality{
		PersonID: p.ID, Framework: "mbti", Result: "INTJ",
		Instrument: "16Personalities", Method: "self_declared_survey",
	}); err != nil {
		t.Fatalf("personality: %v", err)
	}
	if _, err := sens.SetPoliticalLeaning(ctx, domain.PoliticalLeaning{
		PersonID: p.ID, Spectrum: -0.42, LegalBasis: "legitimate_interest",
		InferenceSources: []string{"social_media"},
	}); err != nil {
		t.Fatalf("political leaning: %v", err)
	}

	if _, err := svc.DeactivatePerson(ctx, p.ID, "prepare purge"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	// Measure ONLY the purge transaction (review R-24): statements + wall.
	cctx, counter := pdb.WithQueryCounter(ctx)
	start := time.Now()
	if _, err := svc.PurgePerson(cctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	wall := time.Since(start)
	n := counter.Count()
	t.Logf("person-family purge width baseline: %d statements in %s (personprofile hard-delete + personsensitive crypto-erase)", n, wall)

	if n > purgeWidthBudget {
		t.Fatalf("purge width = %d statements, exceeds budget %d — a step-change regression. "+
			"If intentional, raise purgeWidthBudget and the baseline in docs/architecture/patterns.md.", n, purgeWidthBudget)
	}
}
