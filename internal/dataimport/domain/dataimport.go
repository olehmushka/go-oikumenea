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

// ObjectTypeGeoPlaces is the routing key for the Who's-On-First administrative gazetteer
// (D-GeoPlaces) — the country/region/county/locality tree loaded by the hermenea `wof-sqlite`
// connector. Distinct from geo-countries: a place is keyed by its WOF id, carries geometry, and a
// placetype=country record additionally enriches the matching geo_countries row.
const ObjectTypeGeoPlaces = "geo-places"

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

// GeoPlace is one Who's-On-First gazetteer record decoded from a canonical-envelope record. Optional
// fields use pointers / empty-string for "absent" (the adapter maps them to NULL). GeometryJSON is the
// GeoJSON geometry as raw text (written via ST_GeomFromGeoJSON; "" = no shape). Hierarchy/Concordances
// are raw JSON bytes (nil = absent) landed verbatim in jsonb columns.
type GeoPlace struct {
	WofID        int64
	Placetype    string // country | region | county | locality
	ParentID     *int64
	CountryCode  string // denormalized wof:country (ISO alpha-2; "" if absent)
	Name         string
	Population   *int64
	Hierarchy    []byte
	Concordances []byte
	Status       string // active | retired (retired = WOF mz:is_current=0 / superseded)
	GeometryJSON string
	ISOA3        string // country only, from concordances ("" if absent)
	NumericCode  string // country only, from concordances ("" if absent)
}

// GeoPlaceStore is the port the geo-places upsert handler drives (D-GeoPlaces). Idempotency is keyed on
// source_version: GetVersion returns the row's stored source edition so the handler skips when it
// matches the incoming one, updates when it differs, and inserts when absent — never deletes. A
// placetype=country place additionally enriches the matching geo_countries row (wof_id + geometry) via
// EnrichCountry. Adapters implement it over the caller's transaction.
type GeoPlaceStore interface {
	GetVersion(ctx context.Context, wofID int64) (sourceVersion string, found bool, err error)
	Insert(ctx context.Context, p GeoPlace, prov Provenance) error
	UpdateImport(ctx context.Context, p GeoPlace, prov Provenance) error
	EnrichCountry(ctx context.Context, p GeoPlace, prov Provenance) error
}
