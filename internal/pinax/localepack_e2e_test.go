// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// End-to-end verification of a mounted DATA PACK (D-DataPacks, M54) through the REAL Seeder against a
// migrated Postgres — the boot-autoseed path with `pinax.packs` pointed at deploy/packs. It proves the
// sample locale pack seeds: `deu` becomes a supported locale, its German country translations land, and
// pinax_seed_state records the pack provenance; a second Seed is a no-op for the pack.
//
// Heavy (it also seeds the embedded bundle), so gated behind OIKUMENEA_PINAX_E2E like TestPinaxSeeder.
// Run against a throwaway migrated DB:
//
//	OIKUMENEA_PINAX_E2E=1 OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/m54_e2e?sslmode=disable" \
//	  go test -tags integration -run TestPinaxLocalePack -timeout 300s ./internal/pinax/...
package pinax_test

import (
	"context"
	"os"
	"testing"
)

func TestPinaxLocalePack(t *testing.T) {
	if os.Getenv("OIKUMENEA_PINAX_E2E") == "" {
		t.Skip("set OIKUMENEA_PINAX_E2E=1 to run the locale-pack e2e (seeds the full embedded bundle)")
	}
	ctx := context.Background()
	pool := newPool(t)

	// The composition root passes install.Pinax.Packs; here the repo's sample pack dir (tests run in the
	// package directory, so deploy/packs is two levels up).
	seeder := newSeederWithPacks(t, pool, "../../deploy/packs")

	// First seed applies the embedded bundle + the mounted pack in one dependency order.
	if _, err := seeder.Seed(ctx, false); err != nil {
		t.Fatalf("seed with pack: %v", err)
	}

	// The pack added `deu` as a supported locale (enabled, non-default).
	var enabled, isDefault bool
	if err := pool.QueryRow(ctx,
		`SELECT enabled, is_default FROM oikumenea.i18n_locales WHERE code = 'deu'`,
	).Scan(&enabled, &isDefault); err != nil {
		t.Fatalf("deu locale not seeded by the pack: %v", err)
	}
	if !enabled || isDefault {
		t.Fatalf("deu = {enabled:%v is_default:%v}, want {true false}", enabled, isDefault)
	}

	// Its German country translation landed (DE -> Deutschland), proving the translations overlay ran
	// after the locale + resolved against the migration-seeded country skeleton.
	var text string
	if err := pool.QueryRow(ctx,
		`SELECT text FROM oikumenea.i18n_translations
		  WHERE entity_type = 'country' AND entity_id = 'DE' AND locale = 'deu' AND field = 'name'`,
	).Scan(&text); err != nil {
		t.Fatalf("DE/deu translation not seeded: %v", err)
	}
	if text != "Deutschland" {
		t.Fatalf("DE/deu = %q, want Deutschland", text)
	}

	// pinax_seed_state records the pack provenance for the pack's presets (embedded presets stay NULL).
	var pack string
	if err := pool.QueryRow(ctx,
		`SELECT pack FROM oikumenea.pinax_seed_state WHERE preset = 'locale-deu'`,
	).Scan(&pack); err != nil {
		t.Fatalf("pinax_seed_state has no locale-deu row: %v", err)
	}
	if pack != "locale-deu" {
		t.Fatalf("locale-deu pack provenance = %q, want locale-deu", pack)
	}

	// Second seed: the pack's presets are version-gated no-ops (created 0 for them).
	second, err := seeder.Seed(ctx, false)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if _, ran := second["locale-deu"]; ran {
		t.Fatal("locale-deu re-ran on a warm DB; version gate did not hold")
	}
}
