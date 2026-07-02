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

// ObjectTypeEthnicityScheme is the routing key for the hierarchical ethnicity taxonomy (D-PhysicalIdentity
// amendment, M43) — ethnic groups (+ each group's associated-language & homeland-country ties) fed from
// Wikidata by the hermenea `wikidataethnicities` mapper. Records arrive parent-first (the ethnicity
// `parent_id` FK is RESTRICT); after the batch the handler rebuilds the transitive closure. The catalog is
// plaintext reference data — the person's SELECTION (person_ethnicities) is unaffected.
const ObjectTypeEthnicityScheme = "ethnicity-scheme"

// ObjectTypeReligionScheme is the routing key for the recursive faith taxonomy (D-Religion + D-Pinax,
// M45) — religion_taxa nodes (+ their theism classifications) seeded from the bundled `religions` preset.
// Records arrive parent-first (the taxon `parent_id` FK is RESTRICT); the rank code resolves to a
// religion_taxon_ranks RID in SQL; after the batch the handler rebuilds the closure and re-derives each
// taxon's denormalized root religion_id. The migration already seeds a curated tree — a boot autoseed is
// create-if-absent, so existing taxa are skipped and only genuinely-new nodes are inserted.
const ObjectTypeReligionScheme = "religion-scheme"

// ObjectTypeTranslations is the routing key for the pinax i18n translation overlay (D-Pinax + D-i18n,
// M45): (entity_type, entity_id, field, locale) → text rows for the seeded reference catalogs. Records
// carry the entity's natural key(s); the handler resolves them to the entity_id the read path uses
// (code→RID for languoid/writing_system/religion_taxon/rank_*, the code itself for country/ethnicity_type)
// and inserts create-if-absent. Depends on the entity presets (the rows must exist to resolve).
const ObjectTypeTranslations = "translations"

// ObjectTypeColors is the routing key for the per-domain platform_colors palettes (D-Color + D-Pinax,
// M45) seeded from the bundled `colors` preset. Idempotency is keyed on (domain, code): insert when
// absent, update when name/hex changed, skip otherwise — never deletes. Seeds the eye/hair/vehicle
// palettes plus the rank/religion/ethnicity/country palettes the seeded reference catalogs point at.
const ObjectTypeColors = "colors"

// Record is one object-type-specific record decoded from the canonical envelope (a JSON object). The
// registered handler interprets its own shape.
type Record = map[string]any

// Provenance is stamped on every imported row (the per-row half of the D-DataIngestion lineage; the
// run-level ledger lives in hermenea's own DB).
type Provenance struct {
	Source        string
	SourceVersion string
	ImportedAt    time.Time
	// CreateOnly selects the pinax boot-autoseed semantics (D-Pinax, M45): when true a handler INSERTS
	// absent rows but NEVER updates an existing one (create-if-absent, never-overwrite) — so re-seeding
	// can't clobber operator edits or upstream-corrected data. Default false keeps the hermenea import
	// path's update-on-change behavior (and `oikumenea seed --reconcile`).
	CreateOnly bool
}

// Summary is the per-import outcome: rows created, updated, or skipped (already current — idempotent).
type Summary struct {
	Created int
	Updated int
	Skipped int
}

// GeoCountryEnrichment is the pinax country enrichment payload (D-Pinax, M45): the extra reference
// columns the bundled `countries` preset carries beyond code+name. Applied fill-if-empty — a column
// already set (migration skeleton or the WOF geo-places connector) is never overwritten. All fields are
// optional; a record with none is not enriched. GeometryJSON is a GeoJSON geometry (a low-res country
// border Polygon/MultiPolygon) that fills `geom` and derives `centroid`+`bbox` in SQL; the WOF connector
// later upgrades `geom` to high-res (its EnrichCountry is an unconditional UPDATE). Latitude/Longitude are
// a representative point used as the `centroid` FALLBACK for countries with no bundled polygon (small
// nations absent from the low-res border set), so every country is at least locatable. ColorCode resolves
// the domain='country' palette.
type GeoCountryEnrichment struct {
	ISOA3        string
	NumericCode  string
	GeometryJSON string // GeoJSON geometry text ("" = none)
	Latitude     *float64
	Longitude    *float64
	ColorCode    string
}

// Empty reports whether the enrichment carries nothing to apply (so the handler can skip the call).
func (e GeoCountryEnrichment) Empty() bool {
	return e.ISOA3 == "" && e.NumericCode == "" && e.GeometryJSON == "" &&
		e.Latitude == nil && e.Longitude == nil && e.ColorCode == ""
}

// GeoCountryStore is the port the geo-countries upsert handler drives (the M16 first catalog). Reads
// the existing name to decide create/update/skip; writes stamp provenance. Enrich fills the pinax
// reference columns fill-if-empty (never overwriting). Adapters implement it over the caller's
// transaction so the upsert and its audit row commit together (D-Audit).
type GeoCountryStore interface {
	GetName(ctx context.Context, code string) (name string, found bool, err error)
	Insert(ctx context.Context, code, name string, prov Provenance) error
	UpdateImport(ctx context.Context, code, name string, prov Provenance) error
	Enrich(ctx context.Context, code string, e GeoCountryEnrichment) error
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

// Ethnicity is one ethnicity-scheme record (D-PhysicalIdentity amendment, M43). Code is the
// import/idempotency key; Parent is the parent code ("" = root). Languages carries associated-language
// keys (each a Glottolog code OR an ISO-639-3 code — the mapper projects whichever Wikidata holds);
// Countries carries homeland ISO-3166 alpha-2 codes. Group-level metadata — NOT a person's datum.
type Ethnicity struct {
	Code       string
	Name       string
	Parent     string
	WikidataID string
	Languages  []string
	Countries  []string
}

// EthnicityStore is the port the ethnicity-scheme upsert handler drives. Mirrors LanguoidStore:
// idempotency keyed on source_version, the group's language + country ties replaced on every
// insert/update, and a transitive-closure rebuild after the batch. Never deletes an ethnicity type.
type EthnicityStore interface {
	GetVersion(ctx context.Context, code string) (sourceVersion string, found bool, err error)
	Insert(ctx context.Context, e Ethnicity, prov Provenance) error
	UpdateImport(ctx context.Context, e Ethnicity, prov Provenance) error
	ReplaceLanguages(ctx context.Context, code string, languageKeys []string) error
	ReplaceCountries(ctx context.Context, code string, countryCodes []string) error
	RebuildClosure(ctx context.Context) error
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

// Religion is one religion-scheme record (D-Religion + D-Pinax, M45). Code is the import/idempotency
// key; Parent is the parent taxon code ("" = root religion); RankCode is the level marker
// (religion/branch/tradition/sub_tradition/denomination) resolved to a religion_taxon_ranks RID in SQL.
// Classifications are theism-classification codes tied M:N (unresolved codes silently dropped).
type Religion struct {
	Code            string
	Name            string
	Parent          string
	RankCode        string
	Description     string
	WikidataID      string
	Icon            string
	SortOrder       *int
	Classifications []string
}

// ReligionStore is the port the religion-scheme upsert handler drives (D-Religion + D-Pinax). Mirrors
// EthnicityStore: idempotency keyed on source_version, the taxon's theism classifications replaced on
// every insert/update, and — after the batch — a transitive-closure rebuild followed by re-deriving each
// taxon's denormalized root religion_id (RebuildClosure does both). Never deletes a taxon.
type ReligionStore interface {
	GetVersion(ctx context.Context, code string) (sourceVersion string, found bool, err error)
	Insert(ctx context.Context, r Religion, prov Provenance) error
	UpdateImport(ctx context.Context, r Religion, prov Provenance) error
	ReplaceClassifications(ctx context.Context, code string, classificationCodes []string) error
	RebuildClosure(ctx context.Context) error
}

// Color is one platform_colors palette record (D-Color + D-Pinax, M45). Idempotency is keyed on the
// (Domain, Code) pair; Hex is an optional swatch ("" = none). No provenance columns exist on
// platform_colors — origin='seeded' (set in SQL) marks preset ownership.
type Color struct {
	Domain    string // eye | hair | vehicle | rank | religion | ethnicity | country
	Code      string
	Name      string
	Hex       string
	SortOrder *int
}

// ColorStore is the port the colors upsert handler drives (D-Color + D-Pinax). Reads the existing
// name+hex to decide create/update/skip. Never deletes a color.
type ColorStore interface {
	Get(ctx context.Context, domain, code string) (name, hex string, found bool, err error)
	Insert(ctx context.Context, c Color) error
	UpdateImport(ctx context.Context, c Color) error
}

// Translation is one pinax translation record (D-Pinax + D-i18n, M45): a reference entity's translated
// `field` (default "name") in one or more locales. EntityType selects the read-path slot; Key is the
// entity's natural key (a single code, or a "system/category[/type]/code" path for rank_*); Names is the
// locale→text map. The handler resolves Key→entity_id and inserts each locale create-if-absent.
type Translation struct {
	EntityType string
	Key        string
	Field      string // "" defaults to "name"
	Names      map[string]string
}

// TranslationStore is the port the translations handler drives (D-Pinax + D-i18n). Resolve maps an
// entity's natural key to the entity_id the read path uses (found=false → the entity is not seeded yet,
// so the record is skipped). Upsert writes one (entity_type, entity_id, field, locale)→text row
// create-if-absent (never clobbering an operator-corrected translation).
type TranslationStore interface {
	Resolve(ctx context.Context, entityType, key string) (entityID string, found bool, err error)
	Upsert(ctx context.Context, entityType, entityID, field, locale, text string) error
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
