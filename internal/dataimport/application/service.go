// Package application holds the data-import module's application service (M16 / D-Hermenea): the
// generic POST /import/{objectType} orchestrator the transport calls. It runs the registered upsert
// handler in ONE transaction and records ONE audited Action as a `system` actor (the bulk-ingest !=
// audited-edit boundary). Connectors/mappers/scheduling are NOT here — they live in the hermenea
// companion; this side only receives canonical envelopes and upserts them.
package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/metrics"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// auditSubsystem labels the `system` actor for import writes. An import is always recorded as a
// system-actor Action regardless of who triggered it (D-Hermenea: ingest != edit).
const auditSubsystem = "data-import"

// Import metrics (architecture review R-20), tagged object_type. The 1M-record M49 run measured these
// with ad-hoc test instrumentation; production runs reported nothing. See docs/modules/platform.md.
const (
	metricImportChunkSeconds = "dataimport.chunk_seconds" // per-chunk handler latency (timer)
	metricImportRowsMerged   = "dataimport.rows.merged"   // rows created + updated (counter)
	metricImportRowsSkipped  = "dataimport.rows.skipped"  // rows skipped as unchanged (counter)
)

// Handler applies an object-type's records as an idempotent, non-destructive upsert into its catalog,
// within the caller's transaction, stamping provenance. Registered per object-type at composition time
// (mirrors pkg/events.Bus: adding an importable catalog = registering one handler). chunk places the
// envelope within a chunked run (R-05): single-shot envelopes arrive as {Seq: 1, IsLast: true}, and a
// handler with a batch finalizer runs it on IsLast (see domain.ChunkInfo). A chunk may carry zero
// records (the trailing finalize marker).
type Handler func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error)

// StoreFactory binds the geo-country store port to a command surface (the caller's tx). Injected by
// module.go so the application never imports adapters (mirrors rank's RepositoryFactory).
type StoreFactory func(conn db.DBTX) domain.GeoCountryStore

// GeoPlaceStoreFactory binds the geo-places store port to the caller's tx (D-GeoPlaces).
type GeoPlaceStoreFactory func(conn db.DBTX) domain.GeoPlaceStore

// LanguoidStoreFactory binds the languoid store port to the caller's tx (D-Languages, M18).
type LanguoidStoreFactory func(conn db.DBTX) domain.LanguoidStore

// LanguageScriptStoreFactory binds the language-scripts store port to the caller's tx (D-Languages).
type LanguageScriptStoreFactory func(conn db.DBTX) domain.LanguageScriptStore

// ExternalOrgStoreFactory binds the external-organizations store port to the caller's tx (D-ExternalOrgs).
type ExternalOrgStoreFactory func(conn db.DBTX) domain.ExternalOrgStore

// EthnicityStoreFactory binds the ethnicity-scheme store port to the caller's tx (D-PhysicalIdentity, M43).
type EthnicityStoreFactory func(conn db.DBTX) domain.EthnicityStore

// ReligionStoreFactory binds the religion-scheme store port to the caller's tx (D-Religion + D-Pinax, M45).
type ReligionStoreFactory func(conn db.DBTX) domain.ReligionStore

// ColorStoreFactory binds the colors store port to the caller's tx (D-Color + D-Pinax, M45).
type ColorStoreFactory func(conn db.DBTX) domain.ColorStore

// TranslationStoreFactory binds the translations store port to the caller's tx (D-Pinax + D-i18n, M45).
type TranslationStoreFactory func(conn db.DBTX) domain.TranslationStore

// RegulatorySanctionStoreFactory binds the regulatory-sanctions store port to the caller's tx
// (D-Watchlists, M34).
type RegulatorySanctionStoreFactory func(conn db.DBTX) domain.RegulatorySanctionStore

// Envelope is the application-level canonical envelope (the transport maps the Conjure type onto it).
type Envelope struct {
	ObjectType    string
	Source        string
	SourceVersion string
	Records       []domain.Record
	// CreateOnly requests the pinax boot-autoseed semantics (D-Pinax): insert absent rows, never update
	// an existing one. The pinax seeder sets it true on boot; false (the default, and every hermenea
	// import) keeps update-on-change.
	CreateOnly bool
	// Chunk places the envelope within a chunked run (R-05); the zero value is a single-shot envelope
	// (Import normalizes it to {Seq: 1, IsLast: true} before the handler sees it).
	Chunk domain.ChunkInfo
}

// Service runs imports over a registry of per-object-type handlers. It owns its writes, so it holds the
// pool to open transactions.
type Service struct {
	pool     *pgxpool.Pool
	audit    *auditapp.Service
	handlers map[string]Handler
}

// NewService builds the service with the pool and the audit service every import records into. The
// handler registry starts empty; module.go registers the available object-types.
func NewService(pool *pgxpool.Pool, audit *auditapp.Service) *Service {
	return &Service{pool: pool, audit: audit, handlers: map[string]Handler{}}
}

// Register adds an upsert handler for an object-type (composition-time wiring, before serving).
func (s *Service) Register(objectType string, h Handler) { s.handlers[objectType] = h }

// Import applies the canonical envelope for objectType as a code-keyed idempotent upsert in one
// transaction, then records one `system`-actor audited Action. An unknown object-type or an empty
// source is rejected before any write.
func (s *Service) Import(ctx context.Context, objectType string, env Envelope) (domain.Summary, error) {
	h, ok := s.handlers[objectType]
	if !ok {
		return domain.Summary{}, domain.ErrUnknownObjectType
	}
	if strings.TrimSpace(env.Source) == "" {
		return domain.Summary{}, domain.ErrInvalidRecord
	}
	prov := domain.Provenance{Source: env.Source, SourceVersion: env.SourceVersion, ImportedAt: time.Now().UTC(), CreateOnly: env.CreateOnly}
	chunk := env.Chunk
	if !chunk.Chunked {
		// Single-shot envelope: the whole batch in one chunk, so IsLast-gated finalizers run as before.
		chunk = domain.ChunkInfo{Seq: 1, IsLast: true}
	}
	var sum domain.Summary
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		// R-20: per-object-type chunk-apply latency + rows merged/skipped. One wrap here covers every
		// registered handler (batch and single-shot) uniformly; the closure-rebuild cost of a finalize
		// chunk shows up in the same histogram tagged by object_type.
		start := time.Now()
		out, err := h(ctx, tx, env.Records, prov, chunk)
		otTag := metrics.MustNewTag("object_type", objectType)
		metrics.FromContext(ctx).Timer(metricImportChunkSeconds, otTag).UpdateSince(start)
		if err != nil {
			return err
		}
		reg := metrics.FromContext(ctx)
		reg.Counter(metricImportRowsMerged, otTag).Inc(int64(out.Created + out.Updated))
		reg.Counter(metricImportRowsSkipped, otTag).Inc(int64(out.Skipped))
		sum = out
		details := map[string]any{
			"source":        env.Source,
			"sourceVersion": env.SourceVersion,
			"records":       len(env.Records),
			"summary":       out,
		}
		if chunk.Chunked {
			details["runId"] = chunk.RunID
			details["seq"] = chunk.Seq
			details["isLast"] = chunk.IsLast
		}
		return s.record(ctx, tx, "import."+objectType, objectType, env.Source, details)
	})
	if err != nil {
		return domain.Summary{}, err
	}
	return sum, nil
}

// GeoCountriesHandler builds the geo-countries upsert handler over the injected store factory
// (M16 first catalog). For each record it reads the existing name, inserts when absent, updates only
// when the name changed, and skips otherwise — never deletes. Provenance is stamped on every write.
func GeoCountriesHandler(newStore StoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			code, name, err := geoFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			existing, found, err := store.GetName(ctx, code)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, code, name, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
			case prov.CreateOnly:
				sum.Skipped++ // pinax boot autoseed: never overwrite an existing row's name
			case existing != name:
				if err := store.UpdateImport(ctx, code, name, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
			default:
				sum.Skipped++
			}
			// Pinax country enrichment (D-Pinax, M45): fill-if-empty, independent of the name
			// create/update/skip above — never overwrites a column already set (migration skeleton or the
			// WOF geo-places connector). The hermenea geo-countries path carries only code+name, so its
			// enrichment is empty and this is a no-op there.
			if enr := geoEnrichment(rec); !enr.Empty() {
				if err := store.Enrich(ctx, code, enr); err != nil {
					return domain.Summary{}, err
				}
			}
		}
		return sum, nil
	}
}

// geoFields reads the ISO-3166 alpha-2 `code` + `name` from a record, normalizing the code to upper
// case. A non-2-letter code or empty name is rejected (ErrInvalidRecord).
func geoFields(rec domain.Record) (code, name string, err error) {
	code, _ = rec["code"].(string)
	name, _ = rec["name"].(string)
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	if len(code) != 2 || name == "" {
		return "", "", domain.ErrInvalidRecord
	}
	return code, name, nil
}

// geoEnrichment reads the optional pinax country-enrichment fields from a record (D-Pinax, M45). Absent
// fields fold to ""/nil, so an ordinary hermenea geo-countries record (code+name only) yields an empty
// enrichment (Empty()==true) and no enrichment write.
func geoEnrichment(rec domain.Record) domain.GeoCountryEnrichment {
	e := domain.GeoCountryEnrichment{
		ISOA3:       strings.ToUpper(strings.TrimSpace(recStr(rec["isoA3"]))),
		NumericCode: strings.TrimSpace(recStr(rec["numericCode"])),
		Latitude:    recOptFloat(rec["latitude"]),
		Longitude:   recOptFloat(rec["longitude"]),
		ColorCode:   strings.TrimSpace(recStr(rec["colorCode"])),
	}
	if g := rec["geometry"]; g != nil {
		if b, err := json.Marshal(g); err == nil {
			e.GeometryJSON = string(b)
		}
	}
	return e
}

// GeoPlacesHandler builds the geo-places upsert handler (D-GeoPlaces; set-based per chunk since
// R-05). Idempotency is keyed on source_version: one merge statement inserts absent places, updates
// those whose incoming edition differs from the stored one, and skips the rest — never deletes. The
// touched placetype=country records additionally enrich their geo_countries rows (wof_id + geometry)
// in one pass. Parent references resolve in a second pass over the touched rows, so a parent may
// arrive in the same chunk; ACROSS chunks records must still arrive parent-first (country → region →
// county → locality): the parent_id FK is RESTRICT, so an unresolvable reference fails the whole
// transaction loudly (the connector's paged mapper guarantees this ordering). A wof_id duplicated
// within one chunk merges once (last occurrence wins; the extra counts as skipped).
func GeoPlacesHandler(newStore GeoPlaceStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		var sum domain.Summary
		places := make([]domain.GeoPlace, 0, len(records))
		seen := make(map[int64]int, len(records))
		for _, rec := range records {
			p, err := geoPlaceFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			if i, dup := seen[p.WofID]; dup {
				places[i] = p
				sum.Skipped++
				continue
			}
			seen[p.WofID] = len(places)
			places = append(places, p)
		}
		store := newStore(tx)
		created, updated, err := store.BulkUpsert(ctx, places, prov)
		if err != nil {
			return domain.Summary{}, err
		}
		sum.Created = len(created)
		sum.Updated = len(updated)
		sum.Skipped += len(places) - len(created) - len(updated)
		touched := make(map[int64]bool, len(created)+len(updated))
		for _, id := range created {
			touched[id] = true
		}
		for _, id := range updated {
			touched[id] = true
		}
		var countries []domain.GeoPlace
		for _, p := range places {
			if p.Placetype == "country" && p.CountryCode != "" && touched[p.WofID] {
				countries = append(countries, p)
			}
		}
		if err := store.BulkEnrichCountries(ctx, countries); err != nil {
			return domain.Summary{}, err
		}
		return sum, nil
	}
}

// geoPlaceFields decodes a Who's-On-First record. wofId/name/placetype are required (placetype must be
// one of the four admin types); optional ids/population fold to NULL when absent; geometry is
// re-marshalled to GeoJSON text; hierarchy/concordances are landed as raw JSON; isCurrent=false maps to
// status=retired (non-destructive — WOF supersession is a status flip, not a delete).
func geoPlaceFields(rec domain.Record) (domain.GeoPlace, error) {
	p := domain.GeoPlace{
		WofID:     recInt64(rec["wofId"]),
		Placetype: strings.TrimSpace(recStr(rec["placetype"])),
		Name:      strings.TrimSpace(recStr(rec["name"])),
	}
	if p.WofID == 0 || p.Name == "" || !validPlacetype(p.Placetype) {
		return domain.GeoPlace{}, domain.ErrInvalidRecord
	}
	p.CountryCode = strings.ToUpper(strings.TrimSpace(recStr(rec["countryCode"])))
	p.ParentID = recOptInt64(rec["parentId"])
	p.Population = recOptInt64(rec["population"])
	p.Status = "active"
	if cur, ok := rec["isCurrent"].(bool); ok && !cur {
		p.Status = "retired"
	}
	if g := rec["geometry"]; g != nil {
		b, err := json.Marshal(g)
		if err != nil {
			return domain.GeoPlace{}, domain.ErrInvalidRecord
		}
		p.GeometryJSON = string(b)
	}
	p.Hierarchy = rawJSON(rec["hierarchy"])
	p.Concordances = rawJSON(rec["concordances"])
	p.ISOA3 = strings.ToUpper(strings.TrimSpace(recStr(rec["isoA3"])))
	p.NumericCode = strings.TrimSpace(recStr(rec["numericCode"]))
	return p, nil
}

// LanguageSchemeHandler builds the Glottolog languoid upsert handler (D-Languages, M18). Idempotency is
// keyed on source_version (like geo-places): a languoid is inserted when absent, updated when the
// incoming Glottolog edition differs, and skipped otherwise — never deleted. Records MUST arrive
// parent-first (family → language → dialect): the parent_id FK is RESTRICT, so a forward reference
// fails the whole transaction loudly (the glottolog mapper guarantees this ordering, sequential
// chunks preserve it across chunk boundaries). On every insert/update the languoid's country ties are
// replaced. Once the batch is complete (chunk.IsLast — the single envelope, or a chunked run's final
// chunk) the handler rebuilds the transitive closure and the denormalized family_code in one shot.
func LanguageSchemeHandler(newStore LanguoidStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		var sum domain.Summary
		ls := make([]domain.Languoid, 0, len(records))
		seen := make(map[string]int, len(records))
		for _, rec := range records {
			l, err := languoidFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			if i, dup := seen[l.Code]; dup {
				ls[i] = l
				sum.Skipped++ // a glottocode duplicated within one chunk merges once (last wins)
				continue
			}
			seen[l.Code] = len(ls)
			ls = append(ls, l)
		}
		store := newStore(tx)
		created, updated, err := store.BulkUpsert(ctx, ls, prov)
		if err != nil {
			return domain.Summary{}, err
		}
		sum.Created = len(created)
		sum.Updated = len(updated)
		sum.Skipped += len(ls) - len(created) - len(updated)
		touched := make(map[string]bool, len(created)+len(updated))
		for _, c := range created {
			touched[c] = true
		}
		for _, c := range updated {
			touched[c] = true
		}
		// Replace the touched languoids' country ties set-based (skipped rows — incl. every existing row
		// under pinax CreateOnly — keep their ties, as before).
		if len(touched) > 0 {
			codes := make([]string, 0, len(touched))
			var pairCodes, pairCountries []string
			for _, l := range ls {
				if !touched[l.Code] {
					continue
				}
				codes = append(codes, l.Code)
				for _, cc := range l.Countries {
					if cc == "" {
						continue
					}
					pairCodes = append(pairCodes, l.Code)
					pairCountries = append(pairCountries, cc)
				}
			}
			if err := store.BulkReplaceCountries(ctx, codes, pairCodes, pairCountries); err != nil {
				return domain.Summary{}, err
			}
		}
		// Rebuild the closure + family_code, then reconcile the locale→languoid link (D-i18n) so supported
		// UI locales point at their languoid — only once the batch is complete (chunk.IsLast). Single-shot
		// keeps the touched-gate (skip a full no-op import); a chunked run finalizes unconditionally on its
		// last chunk (the server is stateless across chunks and cannot know whether an earlier one touched).
		if chunk.IsLast && (len(touched) > 0 || chunk.Chunked) {
			if err := store.RebuildClosure(ctx); err != nil {
				return domain.Summary{}, err
			}
			if err := store.ReconcileLocaleLanguages(ctx); err != nil {
				return domain.Summary{}, err
			}
		}
		return sum, nil
	}
}

// languoidFields decodes a Glottolog record. code/level/name are required (level must be one of the
// three Glottolog levels); optional fields fold to ""/nil; status defaults to not_endangered; countries
// are upper-cased ISO alpha-2 codes.
func languoidFields(rec domain.Record) (domain.Languoid, error) {
	l := domain.Languoid{
		Code:      strings.ToLower(strings.TrimSpace(recStr(rec["code"]))),
		Level:     strings.ToLower(strings.TrimSpace(recStr(rec["level"]))),
		Name:      strings.TrimSpace(recStr(rec["name"])),
		Parent:    strings.ToLower(strings.TrimSpace(recStr(rec["parent"]))),
		ISO639_3:  strings.ToLower(strings.TrimSpace(recStr(rec["iso639_3"]))),
		Macroarea: strings.TrimSpace(recStr(rec["macroarea"])),
		Latitude:  recOptFloat(rec["latitude"]),
		Longitude: recOptFloat(rec["longitude"]),
		Status:    strings.TrimSpace(recStr(rec["status"])),
	}
	if l.Code == "" || l.Name == "" || !validLevel(l.Level) {
		return domain.Languoid{}, domain.ErrInvalidRecord
	}
	if l.Status == "" {
		l.Status = "not_endangered"
	}
	for _, c := range recStrList(rec["countries"]) {
		if cc := strings.ToUpper(strings.TrimSpace(c)); cc != "" {
			l.Countries = append(l.Countries, cc)
		}
	}
	return l, nil
}

func validLevel(s string) bool {
	switch s {
	case "family", "language", "dialect":
		return true
	}
	return false
}

// LanguageScriptsHandler builds the CLDR language→writing-system upsert handler (D-Languages, M18). A
// record ties a language (by ISO 639-3) to a writing system (by ISO-15924 code) with an is_primary
// flag. A record whose languoid or writing system does not resolve is skipped (counted, not an error) —
// so the import is resilient to a partly-seeded script catalog and to languages outside the imported
// scheme. Idempotency is on the (languoid, writing-system) pair.
func LanguageScriptsHandler(newStore LanguageScriptStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			iso := strings.ToLower(strings.TrimSpace(recStr(rec["iso639_3"])))
			ws := strings.TrimSpace(recStr(rec["writingSystem"]))
			isPrimary, _ := rec["isPrimary"].(bool)
			if iso == "" || ws == "" {
				return domain.Summary{}, domain.ErrInvalidRecord
			}
			lid, ok, err := store.ResolveLanguoid(ctx, iso)
			if err != nil {
				return domain.Summary{}, err
			}
			if !ok {
				sum.Skipped++
				continue
			}
			wid, ok, err := store.ResolveWritingSystem(ctx, ws)
			if err != nil {
				return domain.Summary{}, err
			}
			if !ok {
				sum.Skipped++
				continue
			}
			cur, found, err := store.GetLinkPrimary(ctx, lid, wid)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.InsertLink(ctx, lid, wid, isPrimary, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
			case prov.CreateOnly:
				sum.Skipped++ // pinax boot autoseed: never overwrite an existing language↔script link
			case cur != isPrimary:
				if err := store.UpdateLink(ctx, lid, wid, isPrimary, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
			default:
				sum.Skipped++
			}
		}
		return sum, nil
	}
}

// EthnicitySchemeHandler builds the ethnicity taxonomy upsert handler (D-PhysicalIdentity amendment, M43).
// Mirrors LanguageSchemeHandler: idempotency keyed on source_version (insert when absent, update when the
// incoming edition differs, skip otherwise — never deletes); records MUST arrive parent-first (parent_id
// FK is RESTRICT); the group's language + country ties are replaced on every insert/update; once the
// batch is complete (chunk.IsLast) the transitive closure is rebuilt once. Group-level reference data —
// the person's declared ethnicity (person_ethnicities) is untouched.
func EthnicitySchemeHandler(newStore EthnicityStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		touched := false
		for _, rec := range records {
			e, err := ethnicityFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			ver, found, err := store.GetVersion(ctx, e.Code)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, e, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
				touched = true
			case prov.CreateOnly:
				sum.Skipped++
				continue // pinax boot autoseed: never overwrite an existing ethnicity type or its ties
			case ver != prov.SourceVersion:
				if err := store.UpdateImport(ctx, e, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
				touched = true
			default:
				sum.Skipped++
				continue // unchanged edition: leave language/country ties as-is
			}
			if err := store.ReplaceLanguages(ctx, e.Code, e.Languages); err != nil {
				return domain.Summary{}, err
			}
			if err := store.ReplaceCountries(ctx, e.Code, e.Countries); err != nil {
				return domain.Summary{}, err
			}
		}
		// Closure rebuild only once the batch is complete (chunk.IsLast): single-shot keeps the
		// touched-gate; a chunked run finalizes unconditionally on its last chunk (see LanguageSchemeHandler).
		if chunk.IsLast && (touched || chunk.Chunked) {
			if err := store.RebuildClosure(ctx); err != nil {
				return domain.Summary{}, err
			}
		}
		return sum, nil
	}
}

// ethnicityFields decodes an ethnicity record. code + name are required; parent/wikidataId fold to "";
// languages are lower-cased keys (glottocode or ISO-639-3), countries upper-cased ISO alpha-2.
func ethnicityFields(rec domain.Record) (domain.Ethnicity, error) {
	e := domain.Ethnicity{
		Code:       strings.ToLower(strings.TrimSpace(recStr(rec["code"]))),
		Name:       strings.TrimSpace(recStr(rec["name"])),
		Parent:     strings.ToLower(strings.TrimSpace(recStr(rec["parent"]))),
		WikidataID: strings.TrimSpace(recStr(rec["wikidataId"])),
	}
	if e.Code == "" || e.Name == "" {
		return domain.Ethnicity{}, domain.ErrInvalidRecord
	}
	for _, l := range recStrList(rec["languages"]) {
		if k := strings.ToLower(strings.TrimSpace(l)); k != "" {
			e.Languages = append(e.Languages, k)
		}
	}
	for _, c := range recStrList(rec["countries"]) {
		if cc := strings.ToUpper(strings.TrimSpace(c)); cc != "" {
			e.Countries = append(e.Countries, cc)
		}
	}
	return e, nil
}

// ReligionSchemeHandler builds the faith-taxonomy upsert handler (D-Religion + D-Pinax, M45). Mirrors
// EthnicitySchemeHandler: idempotency keyed on source_version (insert when absent, update when the
// incoming edition differs, skip otherwise — never deletes); records MUST arrive parent-first (parent_id
// FK is RESTRICT); the taxon's theism classifications are replaced on every insert/update; once the
// batch is complete (chunk.IsLast) the closure is rebuilt and each taxon's denormalized root
// religion_id re-derived. The
// migration seeds a curated tree, so a boot autoseed (CreateOnly) skips existing taxa and inserts only
// genuinely-new nodes.
func ReligionSchemeHandler(newStore ReligionStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		touched := false
		for _, rec := range records {
			r, err := religionFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			ver, found, err := store.GetVersion(ctx, r.Code)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, r, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
				touched = true
			case prov.CreateOnly:
				sum.Skipped++
				continue // pinax boot autoseed: never overwrite an existing taxon or its ties
			case ver != prov.SourceVersion:
				if err := store.UpdateImport(ctx, r, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
				touched = true
			default:
				sum.Skipped++
				continue // unchanged edition: leave classifications as-is
			}
			if err := store.ReplaceClassifications(ctx, r.Code, r.Classifications); err != nil {
				return domain.Summary{}, err
			}
		}
		// Closure rebuild only once the batch is complete (chunk.IsLast): single-shot keeps the
		// touched-gate; a chunked run finalizes unconditionally on its last chunk (see LanguageSchemeHandler).
		if chunk.IsLast && (touched || chunk.Chunked) {
			if err := store.RebuildClosure(ctx); err != nil {
				return domain.Summary{}, err
			}
		}
		return sum, nil
	}
}

// religionFields decodes a religion-scheme record. code + name + rank are required (rank is the level
// marker resolved to a religion_taxon_ranks RID in SQL); parent/description/wikidataId/icon fold to "";
// classifications are lower-cased theism codes.
func religionFields(rec domain.Record) (domain.Religion, error) {
	r := domain.Religion{
		Code:        strings.ToLower(strings.TrimSpace(recStr(rec["code"]))),
		Name:        strings.TrimSpace(recStr(rec["name"])),
		Parent:      strings.ToLower(strings.TrimSpace(recStr(rec["parent"]))),
		RankCode:    strings.ToLower(strings.TrimSpace(recStr(rec["rank"]))),
		Description: strings.TrimSpace(recStr(rec["description"])),
		WikidataID:  strings.TrimSpace(recStr(rec["wikidataId"])),
		Icon:        strings.TrimSpace(recStr(rec["icon"])),
		SortOrder:   recOptInt(rec["sortOrder"]),
	}
	if r.Code == "" || r.Name == "" || r.RankCode == "" {
		return domain.Religion{}, domain.ErrInvalidRecord
	}
	for _, c := range recStrList(rec["classifications"]) {
		if k := strings.ToLower(strings.TrimSpace(c)); k != "" {
			r.Classifications = append(r.Classifications, k)
		}
	}
	return r, nil
}

// ColorsHandler builds the platform_colors palette upsert handler (D-Color + D-Pinax, M45). Idempotency
// is keyed on the (domain, code) pair: insert when absent, update when name/hex changed, skip otherwise —
// never deletes. On a pinax boot autoseed (CreateOnly) an existing color is never overwritten.
func ColorsHandler(newStore ColorStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			c, err := colorFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			name, hex, found, err := store.Get(ctx, c.Domain, c.Code)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, c); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
			case prov.CreateOnly:
				sum.Skipped++ // pinax boot autoseed: never overwrite an existing color
			case name != c.Name || hex != c.Hex:
				if err := store.UpdateImport(ctx, c); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
			default:
				sum.Skipped++
			}
		}
		return sum, nil
	}
}

// colorFields decodes a colors record. domain + code + name are required; hex folds to "" when absent.
func colorFields(rec domain.Record) (domain.Color, error) {
	c := domain.Color{
		Domain:    strings.ToLower(strings.TrimSpace(recStr(rec["domain"]))),
		Code:      strings.ToLower(strings.TrimSpace(recStr(rec["code"]))),
		Name:      strings.TrimSpace(recStr(rec["name"])),
		Hex:       strings.TrimSpace(recStr(rec["hex"])),
		SortOrder: recOptInt(rec["sortOrder"]),
	}
	if c.Domain == "" || c.Code == "" || c.Name == "" {
		return domain.Color{}, domain.ErrInvalidRecord
	}
	return c, nil
}

// TranslationsHandler builds the pinax i18n translation-overlay handler (D-Pinax + D-i18n, M45). Each
// record carries an entity's natural key + a locale→text map for one field; the handler resolves the key
// to the entity_id the read path uses (skipping records whose entity is not seeded yet — resilient to a
// partly-seeded plane) and writes each locale create-if-absent. It runs after the entity presets (the
// `translations` preset dependsOn them). CreateOnly vs reconcile is moot: writes are always
// create-if-absent (the store has no provenance column, so a re-seed never clobbers an operator edit).
func TranslationsHandler(newStore TranslationStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, _ domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			t, err := translationFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			entityID, ok, err := store.Resolve(ctx, t.EntityType, t.Key)
			if err != nil {
				return domain.Summary{}, err
			}
			if !ok {
				sum.Skipped++ // entity not seeded (yet) — leave its translations for a later run
				continue
			}
			wrote := false
			for locale, text := range t.Names {
				if strings.TrimSpace(text) == "" {
					continue
				}
				if err := store.Upsert(ctx, t.EntityType, entityID, t.Field, locale, text); err != nil {
					return domain.Summary{}, err
				}
				wrote = true
			}
			if wrote {
				sum.Created++
			} else {
				sum.Skipped++
			}
		}
		return sum, nil
	}
}

// LocaleStoreFactory binds the locale store port to the caller's tx (D-DataPacks + D-i18n, M54).
type LocaleStoreFactory func(conn db.DBTX) domain.LocaleStore

// LocalesHandler builds the supported-locale import handler (D-DataPacks, M54) — the path a LOCALE
// PACK's `locales` preset uses to add i18n_locales rows before its translation overlays. Each record is
// {code, name}; the code (ISO 639-3) is lower-cased and added create-if-absent. An already-supported
// code is counted skipped and left untouched, so a pack never flips an operator's enabled/is_default.
func LocalesHandler(newStore LocaleStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, _ domain.Provenance, _ domain.ChunkInfo) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			code := strings.ToLower(strings.TrimSpace(recStr(rec["code"])))
			name := strings.TrimSpace(recStr(rec["name"]))
			if code == "" || name == "" {
				return domain.Summary{}, domain.ErrInvalidRecord
			}
			created, err := store.Insert(ctx, code, name)
			if err != nil {
				return domain.Summary{}, err
			}
			if created {
				sum.Created++
			} else {
				sum.Skipped++
			}
		}
		return sum, nil
	}
}

// translationFields decodes a translations record. entityType + key + a non-empty names map are required;
// field defaults to "name".
func translationFields(rec domain.Record) (domain.Translation, error) {
	t := domain.Translation{
		EntityType: strings.TrimSpace(recStr(rec["entityType"])),
		Key:        strings.TrimSpace(recStr(rec["key"])),
		Field:      strings.TrimSpace(recStr(rec["field"])),
		Names:      map[string]string{},
	}
	if t.Field == "" {
		t.Field = "name"
	}
	if m, ok := rec["names"].(map[string]any); ok {
		for locale, v := range m {
			if s, ok := v.(string); ok {
				t.Names[strings.ToLower(strings.TrimSpace(locale))] = s
			}
		}
	}
	if t.EntityType == "" || t.Key == "" || len(t.Names) == 0 {
		return domain.Translation{}, domain.ErrInvalidRecord
	}
	return t, nil
}

// recOptInt reads an optional JSON number as *int (nil when absent/null/non-numeric).
func recOptInt(v any) *int {
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i
	case int:
		return &n
	case int64:
		i := int(n)
		return &i
	case json.Number:
		if i, err := n.Int64(); err == nil {
			x := int(i)
			return &x
		}
	}
	return nil
}

// ExternalOrgsHandler builds the external-organizations upsert handler (D-ExternalOrgs, M30;
// set-based per chunk since R-05). One merge statement resolves each record's kind against the
// external_org_kinds catalog (a record whose kind does not resolve is skipped — the import is
// resilient to mapping gaps) and keys idempotency on the Wikidata id: insert when absent, update only
// when the name changed, skip otherwise — never deletes. Imported rows are stamped source=imported +
// as_of=ImportedAt; the country (ISO alpha-2) resolves to geo_countries in SQL. A Wikidata id
// duplicated within one chunk merges once (last occurrence wins; the extra counts as skipped).
func ExternalOrgsHandler(newStore ExternalOrgStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		var sum domain.Summary
		orgs := make([]domain.ExternalOrg, 0, len(records))
		seen := make(map[string]int, len(records))
		for _, rec := range records {
			o, err := externalOrgFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			if i, dup := seen[o.WikidataID]; dup {
				orgs[i] = o
				sum.Skipped++
				continue
			}
			seen[o.WikidataID] = len(orgs)
			orgs = append(orgs, o)
		}
		created, updated, err := newStore(tx).BulkUpsert(ctx, orgs, prov)
		if err != nil {
			return domain.Summary{}, err
		}
		sum.Created = created
		sum.Updated = updated
		sum.Skipped += len(orgs) - created - updated
		return sum, nil
	}
}

// externalOrgFields decodes an external-organization record. wikidataId + name are required; kind
// defaults to "other" when absent; country is an upper-cased ISO alpha-2 ("" when absent).
func externalOrgFields(rec domain.Record) (domain.ExternalOrg, error) {
	o := domain.ExternalOrg{
		WikidataID:  strings.TrimSpace(recStr(rec["wikidataId"])),
		Name:        strings.TrimSpace(recStr(rec["name"])),
		KindCode:    strings.TrimSpace(recStr(rec["kind"])),
		CountryCode: strings.ToUpper(strings.TrimSpace(recStr(rec["country"]))),
	}
	if o.WikidataID == "" || o.Name == "" {
		return domain.ExternalOrg{}, domain.ErrInvalidRecord
	}
	if o.KindCode == "" {
		o.KindCode = "other"
	}
	return o, nil
}

// RegulatorySanctionsHandler builds the person regulatory-sanction upsert handler (D-Watchlists, M34;
// set-based per chunk since R-05) — a person-scoped import target. One merge statement resolves each
// record's person inline (an unresolved person RID is skipped, non-destructive; a personId that is
// not even a canonical uuid is pre-dropped here for the same reason) and keys idempotency on
// (person, externalId): insert when absent, update when a comparable field changed, skip an unchanged
// re-import (or any existing row under pinax CreateOnly) — never deletes. A (person, externalId)
// duplicated within one chunk merges once (last occurrence wins; the extra counts as skipped).
func RegulatorySanctionsHandler(newStore RegulatorySanctionStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance, chunk domain.ChunkInfo) (domain.Summary, error) {
		var sum domain.Summary
		ss := make([]domain.RegulatorySanction, 0, len(records))
		seen := make(map[string]int, len(records))
		for _, rec := range records {
			s, err := regulatorySanctionFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			if _, err := uuid.Parse(s.PersonID); err != nil {
				sum.Skipped++ // not a canonical person RID → can never resolve
				continue
			}
			key := s.PersonID + "\x00" + s.ExternalID
			if i, dup := seen[key]; dup {
				ss[i] = s
				sum.Skipped++
				continue
			}
			seen[key] = len(ss)
			ss = append(ss, s)
		}
		created, updated, err := newStore(tx).BulkUpsert(ctx, ss, prov)
		if err != nil {
			return domain.Summary{}, err
		}
		sum.Created = created
		sum.Updated = updated
		sum.Skipped += len(ss) - created - updated
		return sum, nil
	}
}

// regulatorySanctionFields decodes a regulatory-sanction record. personId + regulator + externalId are
// required (externalId is the idempotency key); actionType/status fold to their table defaults so the
// idempotency comparison matches the stored effective values.
func regulatorySanctionFields(rec domain.Record) (domain.RegulatorySanction, error) {
	s := domain.RegulatorySanction{
		PersonID:     strings.TrimSpace(recStr(rec["personId"])),
		Regulator:    strings.TrimSpace(recStr(rec["regulator"])),
		ActionType:   strings.TrimSpace(recStr(rec["actionType"])),
		Amount:       recOptFloat(rec["amount"]),
		Currency:     strings.TrimSpace(recStr(rec["currency"])),
		Status:       strings.TrimSpace(recStr(rec["status"])),
		SanctionDate: strings.TrimSpace(recStr(rec["sanctionDate"])),
		SourceURL:    strings.TrimSpace(recStr(rec["sourceUrl"])),
		ExternalID:   strings.TrimSpace(recStr(rec["externalId"])),
	}
	if s.PersonID == "" || s.Regulator == "" || s.ExternalID == "" {
		return domain.RegulatorySanction{}, domain.ErrInvalidRecord
	}
	if s.ActionType == "" {
		s.ActionType = "other"
	}
	if s.Status == "" {
		s.Status = "active"
	}
	return s, nil
}

// recOptFloat reads an optional JSON number as *float64 (nil when absent/null/non-numeric).
func recOptFloat(v any) *float64 {
	switch n := v.(type) {
	case float64:
		return &n
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return &f
		}
	}
	return nil
}

// recStrList reads a JSON array of strings (nil/non-array → empty).
func recStrList(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func validPlacetype(s string) bool {
	switch s {
	case "country", "region", "county", "locality":
		return true
	}
	return false
}

func recStr(v any) string { s, _ := v.(string); return s }

// recInt64 reads a JSON number (decoded as float64) as int64; WOF ids fit exactly within float64's
// 53-bit mantissa.
func recInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

// recOptInt64 returns nil for an absent/null field, else the parsed int64 (0 is treated as absent by
// the NULLIF in the queries — neither a WOF id nor a population is ever a real 0).
func recOptInt64(v any) *int64 {
	if v == nil {
		return nil
	}
	i := recInt64(v)
	return &i
}

// rawJSON re-marshals a nested object to raw JSON bytes for a jsonb column (nil = absent → NULL).
func rawJSON(v any) []byte {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// inTx runs fn in a transaction, committing on success and rolling back on any error (so a failing
// upsert reverts the whole import and its audit row — all-or-nothing).
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// record writes the audited Action for an import in the same transaction (D-Audit), as a `system`
// actor (ingest != edit).
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:        rid,
		ActorType: auditdomain.ActorSystem,
		Subsystem: auditSubsystem,
		// Name the machine that imported, when one did (M51 / D-ServiceIdentities). Empty for
		// in-process callers (the pinax boot autoseeder, the `oikumenea seed` CLI), which carry no
		// principal — subsystem alone still identifies them.
		ActorPrincipalID: authn.PrincipalID(ctx),
		Action:           action,
		TargetType:       targetType,
		TargetID:         targetID,
		RequestID:        requestID(ctx),
		After:            toJSON(after),
		Outcome:          auditdomain.OutcomeSuccess,
	})
}

// mintActionRID mints an Action RID (new_id(app, service, kind) — kind 0 placeholder, as the other
// modules do for audit Action ids).
func mintActionRID(ctx context.Context, tx pgx.Tx) (string, error) {
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(5, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
