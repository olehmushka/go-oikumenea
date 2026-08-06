// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package health holds the platform readiness-gating reporters (witchcraft-go-health;
// docs/modules/platform.md). Diagnostic-only reporters (e.g. closure-drift, D-ClosureDriftHealth)
// arrive with their owning modules and must NOT gate readiness.
package health

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// requiredExtensions are the operator-DB prerequisites the spatial features need: PostGIS for the WOF
// gazetteer (D-GeoPlaces) + the Location point model + radius/bbox queries (D-Location). MGRS is derived
// in the application (pure Go), so no h3-pg or other extension is required — the stock postgis image
// suffices. Readiness refuses if PostGIS is absent, catching a non-PostGIS image at boot.
var requiredExtensions = []string{"postgis"}

// ReadinessSource gates /status/readiness on three conditions (upgrade-safety.md):
//   - the operator database is reachable,
//   - its applied schema revision matches what this binary expects (refuse on newer/unknown), and
//   - the required PostGIS extension is installed.
//
// It implements witchcraft-go-health status.Source: Status() returns 200 when ready, 503 otherwise.
type ReadinessSource struct {
	pool *pgxpool.Pool
}

// NewReadinessSource builds the readiness gate over the given pool.
func NewReadinessSource(pool *pgxpool.Pool) ReadinessSource {
	return ReadinessSource{pool: pool}
}

type readinessMetadata struct {
	Ready          bool   `json:"ready"`
	Reason         string `json:"reason,omitempty"`
	SchemaRevision string `json:"schemaRevision,omitempty"`
	ExpectedSchema string `json:"expectedSchemaRevision"`
}

// Status reports readiness. Signature satisfies witchcraft-go-health status.Source.
func (r ReadinessSource) Status() (int, interface{}) {
	ctx := context.Background()
	meta := readinessMetadata{ExpectedSchema: db.ExpectedSchemaRevision}

	if err := r.pool.Ping(ctx); err != nil {
		meta.Reason = "database unreachable"
		return http.StatusServiceUnavailable, meta
	}

	revision, err := db.ReadSchemaRevision(ctx, r.pool)
	if err != nil {
		meta.Reason = "schema_version unreadable"
		return http.StatusServiceUnavailable, meta
	}
	meta.SchemaRevision = revision

	if revision != db.ExpectedSchemaRevision {
		// DB is newer/unknown or older than this binary understands — refuse readiness rather
		// than risk writing against an unfamiliar schema.
		meta.Reason = "schema revision mismatch"
		return http.StatusServiceUnavailable, meta
	}

	if missing, err := r.missingExtension(ctx); err != nil {
		meta.Reason = "extension check failed"
		return http.StatusServiceUnavailable, meta
	} else if missing != "" {
		// The operator DB lacks PostGIS (e.g. a plain postgres image) — refuse readiness rather than
		// fail at the first spatial write.
		meta.Reason = "missing extension: " + missing
		return http.StatusServiceUnavailable, meta
	}

	meta.Ready = true
	return http.StatusOK, meta
}

// missingExtension returns the name of the first required extension absent from pg_extension, or "" if
// all are present.
func (r ReadinessSource) missingExtension(ctx context.Context) (string, error) {
	for _, ext := range requiredExtensions {
		var present bool
		if err := r.pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)", ext).Scan(&present); err != nil {
			return "", err
		}
		if !present {
			return ext, nil
		}
	}
	return "", nil
}
