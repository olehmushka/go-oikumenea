// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package pinax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/application"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/domain"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

// Importer is the slice of the data-import application service the seeder needs (so tests can fake it,
// and pinax never imports the transport). *application.Service satisfies it. It is the default path:
// flat canonical records upserted in one shared transaction by a registered handler.
type Importer interface {
	Import(ctx context.Context, objectType string, env application.Envelope) (domain.Summary, error)
}

// NativeImporter applies a preset that does NOT fit the flat canonical-record path — it owns its own
// transaction and nested decoding (e.g. ranks via rank.Service.ImportPreset, whose system→category→
// type→rank subtree can't be a flat record list under one caller-provided tx). The composition root
// registers one per objectType, keeping pinax decoupled from those domain modules. `reconcile` mirrors
// the seeder mode (false = boot create-if-absent).
type NativeImporter func(ctx context.Context, records []map[string]any, reconcile bool) (domain.Summary, error)

// Seeder self-seeds the embedded pinax presets into oikumenea's DB (D-Pinax, M45). It holds the pool
// only to read/write the pinax_seed_state version-gate marker; all writes go through the Importer or a
// per-objectType NativeImporter.
type Seeder struct {
	pool    *pgxpool.Pool
	imp     Importer
	native  map[string]NativeImporter
	presets []Preset
}

// NewSeeder loads + dependency-orders the embedded presets AND any presets under the operator-mounted
// packs directory (packsDir "" = embedded-only; D-DataPacks, M54), and binds them to the import service
// plus any native importers (keyed by objectType; nil is fine). A malformed/cyclic bundle, a preset
// name collision across the bundle and packs, or a preset whose objectType has no importer, fails here
// (loudly, at composition) rather than half-seeding at boot.
func NewSeeder(pool *pgxpool.Pool, imp Importer, native map[string]NativeImporter, packsDir string) (*Seeder, error) {
	presets, err := loadPresets(packsDir)
	if err != nil {
		return nil, err
	}
	return &Seeder{pool: pool, imp: imp, native: native, presets: presets}, nil
}

// Seed applies every preset in dependency order. With reconcile=false (boot autoseed) it is
// create-if-absent + version-gated: a preset whose applied source_version already matches is skipped
// whole (the O(#presets) warm-boot no-op), and existing rows are never overwritten. With reconcile=true
// (`oikumenea seed --reconcile`) every preset runs with update-on-change. Each preset's outcome is
// recorded in pinax_seed_state. Returns the per-preset summaries actually run.
func (s *Seeder) Seed(ctx context.Context, reconcile bool) (map[string]domain.Summary, error) {
	logger := svc1log.FromContext(ctx)
	out := make(map[string]domain.Summary, len(s.presets))
	for _, p := range s.presets {
		applied, ok, err := s.appliedVersion(ctx, p.Name)
		if err != nil {
			return nil, err
		}
		if !reconcile && ok && applied == p.SourceVersion {
			logger.Debug("pinax preset up to date, skipping",
				svc1log.SafeParam("preset", p.Name), svc1log.SafeParam("version", p.SourceVersion))
			continue
		}
		var sum domain.Summary
		if ni, ok := s.native[p.ObjectType]; ok {
			sum, err = ni(ctx, p.Records, reconcile)
		} else {
			recs := make([]domain.Record, len(p.Records))
			for i := range p.Records {
				recs[i] = domain.Record(p.Records[i])
			}
			sum, err = s.imp.Import(ctx, p.ObjectType, application.Envelope{
				ObjectType:    p.ObjectType,
				Source:        p.Source,
				SourceVersion: p.SourceVersion,
				Records:       recs,
				CreateOnly:    !reconcile,
			})
		}
		if err != nil {
			return nil, fmt.Errorf("pinax seed %q (%s): %w", p.Name, p.ObjectType, err)
		}
		if err := s.markApplied(ctx, p, sum); err != nil {
			return nil, err
		}
		out[p.Name] = sum
		logger.Info("pinax preset seeded",
			svc1log.SafeParam("preset", p.Name), svc1log.SafeParam("version", p.SourceVersion),
			svc1log.SafeParam("created", sum.Created), svc1log.SafeParam("updated", sum.Updated),
			svc1log.SafeParam("skipped", sum.Skipped))
	}
	return out, nil
}

// appliedVersion reads the source_version last applied for a preset (ok=false when never seeded).
func (s *Seeder) appliedVersion(ctx context.Context, preset string) (string, bool, error) {
	var v string
	err := s.pool.QueryRow(ctx,
		"SELECT source_version FROM oikumenea.pinax_seed_state WHERE preset = $1", preset).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// markApplied upserts the version-gate marker after a preset seeds. Run outside the import's own
// transaction: a crash between the two just re-runs the (idempotent, create-if-absent) import next boot.
func (s *Seeder) markApplied(ctx context.Context, p Preset, sum domain.Summary) error {
	summary, err := json.Marshal(sum)
	if err != nil {
		return err
	}
	var pack any
	if p.Pack != "" {
		pack = p.Pack
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO oikumenea.pinax_seed_state (preset, source_version, applied_at, summary, pack)
		VALUES ($1, $2, now(), $3, $4)
		ON CONFLICT (preset) DO UPDATE
		  SET source_version = EXCLUDED.source_version, applied_at = now(), summary = EXCLUDED.summary,
		      pack = EXCLUDED.pack`,
		p.Name, p.SourceVersion, summary, pack)
	return err
}
