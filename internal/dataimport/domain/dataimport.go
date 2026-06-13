// Package domain holds the data-import module's framework-free core (M16 / D-Hermenea): the canonical
// record/provenance/summary types and the per-object-type upsert ports. oikumenea is the OWNER of the
// import endpoint; the connectors/mappers/scheduler live out of process in the hermenea companion
// (docs/modules/hermenea.md). An import is a code-keyed, idempotent, NON-DESTRUCTIVE upsert: it never
// deletes, and stamps provenance on every imported row.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrUnknownObjectType is returned when no upsert handler is registered for the requested type.
	ErrUnknownObjectType = errors.New("unknown import object-type")
	// ErrEnvelopeMismatch is returned when the envelope's objectType differs from the path object-type.
	ErrEnvelopeMismatch = errors.New("envelope object-type does not match path object-type")
	// ErrInvalidRecord is returned when a record (or the envelope) is malformed for its object-type.
	ErrInvalidRecord = errors.New("invalid import record")
)

// ObjectTypeGeoCountries is the routing key for the ISO-3166 country catalog — M16's first importable
// object-type (an existing reference catalog needing no new domain module).
const ObjectTypeGeoCountries = "geo-countries"

// Record is one object-type-specific record decoded from the canonical envelope (a JSON object). The
// registered handler interprets its own shape.
type Record = map[string]any

// Provenance is stamped on every imported row (the per-row half of the D-DataIngestion lineage; the
// run-level ledger lives in hermenea's own DB).
type Provenance struct {
	Source        string
	SourceVersion string
	ImportedAt    time.Time
}

// Summary is the per-import outcome: rows created, updated, or skipped (already current — idempotent).
type Summary struct {
	Created int
	Updated int
	Skipped int
}

// GeoCountryStore is the port the geo-countries upsert handler drives (the M16 first catalog). Reads
// the existing name to decide create/update/skip; writes stamp provenance. Adapters implement it over
// the caller's transaction so the upsert and its audit row commit together (D-Audit).
type GeoCountryStore interface {
	GetName(ctx context.Context, code string) (name string, found bool, err error)
	Insert(ctx context.Context, code, name string, prov Provenance) error
	UpdateImport(ctx context.Context, code, name string, prov Provenance) error
}
