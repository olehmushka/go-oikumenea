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

// ObjectTypeLanguageScheme is the routing key for the Glottolog languoid forest (D-Languages, M18) —
// the family/language/dialect tree (+ each languoid's country ties) loaded by the hermenea `glottolog`
// mapper. Records arrive parent-first (the languoid `parent_id` FK is RESTRICT); after the batch the
// handler rebuilds the transitive closure and the denormalized `family_code`.
const ObjectTypeLanguageScheme = "language-scheme"

// ObjectTypeLanguageScripts is the routing key for the CLDR language→writing-system links
// (D-Languages, M18): which ISO-15924 script(s) a language is written in, and which is primary. The
// languoids (language-scheme) and the ISO-15924 `writing_systems` catalog (migration-seeded) must
// pre-exist; a link whose languoid or script does not resolve is skipped (not an error).
const ObjectTypeLanguageScripts = "language-scripts"

// ObjectTypeExternalOrgs is the routing key for the external-organizations registry (D-ExternalOrgs,
// M30) — parties/government bodies/military/NGOs/registrants fed from Wikidata / public registries by
// the hermenea `wikidataorgs` mapper. Records are keyed by their Wikidata Q-id; an unknown `kind` is
// skipped (the import is resilient to mapping gaps). Per-row provenance lands in the attribution columns
// (source=imported + as_of), not a separate provenance column-set.
const ObjectTypeExternalOrgs = "external-organizations"

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

// Languoid is one Glottolog node decoded from a canonical-envelope record (D-Languages, M18). Parent is
// carried as the parent's glottocode (resolved to its RID in SQL on upsert, parent-first ordering
// required). Optional fields use pointers / empty-string for "absent". Countries are ISO-3166 alpha-2
// codes (CLDF Country_IDs), resolved to geo_countries RIDs in SQL.
type Languoid struct {
	Code      string // glottocode (8-char), the import/idempotency key
	Level     string // family | language | dialect
	Name      string
	Parent    string // parent glottocode ("" = root)
	ISO639_3  string // optional ISO 639-3 ("" = absent)
	Macroarea string
	Latitude  *float64
	Longitude *float64
	Status    string   // AES endangerment (not_endangered…extinct); "" defaults to not_endangered
	Countries []string // ISO-3166 alpha-2 country codes
}

// LanguoidStore is the port the language-scheme upsert handler drives (D-Languages). Idempotency is
// keyed on source_version (like geo-places). The languoid's country ties are replaced on every
// insert/update (ReplaceCountries). After the whole batch, RebuildClosure recomputes the transitive
// closure and the denormalized family_code in SQL, and ReconcileLocaleLanguages links each supported
// UI locale to the languoid carrying its ISO-639-3 code (D-i18n). Never deletes a languoid.
type LanguoidStore interface {
	GetVersion(ctx context.Context, code string) (sourceVersion string, found bool, err error)
	Insert(ctx context.Context, l Languoid, prov Provenance) error
	UpdateImport(ctx context.Context, l Languoid, prov Provenance) error
	ReplaceCountries(ctx context.Context, code string, countryCodes []string) error
	RebuildClosure(ctx context.Context) error
	ReconcileLocaleLanguages(ctx context.Context) error
}

// ExternalOrg is one external-organization record decoded from a canonical-envelope record
// (D-ExternalOrgs, M30). WikidataID is the idempotency key; KindCode resolves to the external_org_kinds
// catalog; CountryCode is an ISO-3166 alpha-2 code resolved to geo_countries in SQL ("" = none).
type ExternalOrg struct {
	WikidataID  string
	Name        string
	KindCode    string
	CountryCode string // ISO alpha-2; "" = none
}

// ExternalOrgStore is the port the external-organizations upsert handler drives (D-ExternalOrgs). The
// handler resolves the kind (skipping records whose kind is unknown), then keys idempotency on the
// Wikidata id: insert when absent, update when the name changed, skip otherwise — never deletes.
// Imported rows are stamped source=imported + as_of=ImportedAt in the attribution columns.
type ExternalOrgStore interface {
	ResolveKind(ctx context.Context, code string) (id string, found bool, err error)
	GetByWikidata(ctx context.Context, wikidataID string) (name string, found bool, err error)
	Insert(ctx context.Context, kindID string, o ExternalOrg, prov Provenance) error
	UpdateImport(ctx context.Context, kindID string, o ExternalOrg, prov Provenance) error
}

// LanguageScriptStore is the port the language-scripts upsert handler drives (D-Languages). A link ties
// a languoid (resolved by its ISO 639-3 code) to a writing system (resolved by its ISO-15924 code);
// either failing to resolve makes the handler skip the record. Idempotency is on the (languoid,
// writing-system) pair: insert when absent, update when is_primary changed, skip otherwise.
type LanguageScriptStore interface {
	ResolveLanguoid(ctx context.Context, iso639_3 string) (id string, found bool, err error)
	ResolveWritingSystem(ctx context.Context, code string) (id string, found bool, err error)
	GetLinkPrimary(ctx context.Context, languoidID, writingSystemID string) (isPrimary bool, found bool, err error)
	InsertLink(ctx context.Context, languoidID, writingSystemID string, isPrimary bool, prov Provenance) error
	UpdateLink(ctx context.Context, languoidID, writingSystemID string, isPrimary bool, prov Provenance) error
}
