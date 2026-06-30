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
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// auditSubsystem labels the `system` actor for import writes. An import is always recorded as a
// system-actor Action regardless of who triggered it (D-Hermenea: ingest != edit).
const auditSubsystem = "data-import"

// Handler applies an object-type's records as an idempotent, non-destructive upsert into its catalog,
// within the caller's transaction, stamping provenance. Registered per object-type at composition time
// (mirrors pkg/events.Bus: adding an importable catalog = registering one handler).
type Handler func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error)

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

// Envelope is the application-level canonical envelope (the transport maps the Conjure type onto it).
type Envelope struct {
	ObjectType    string
	Source        string
	SourceVersion string
	Records       []domain.Record
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
	prov := domain.Provenance{Source: env.Source, SourceVersion: env.SourceVersion, ImportedAt: time.Now().UTC()}
	var sum domain.Summary
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		out, err := h(ctx, tx, env.Records, prov)
		if err != nil {
			return err
		}
		sum = out
		return s.record(ctx, tx, "import."+objectType, objectType, env.Source, map[string]any{
			"source":        env.Source,
			"sourceVersion": env.SourceVersion,
			"records":       len(env.Records),
			"summary":       out,
		})
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
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error) {
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
			case existing != name:
				if err := store.UpdateImport(ctx, code, name, prov); err != nil {
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

// GeoPlacesHandler builds the geo-places upsert handler (D-GeoPlaces). Idempotency is keyed on
// source_version: a place is inserted when absent, updated when the incoming edition differs from the
// stored one, and skipped otherwise — never deleted. A placetype=country record additionally enriches
// the matching geo_countries row (wof_id + geometry) in place. Records MUST arrive parent-first
// (country → region → county → locality): the parent_id FK is RESTRICT, so a forward reference fails
// the whole transaction loudly (the connector's paged mapper guarantees this ordering).
func GeoPlacesHandler(newStore GeoPlaceStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			p, err := geoPlaceFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			ver, found, err := store.GetVersion(ctx, p.WofID)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, p, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
			case ver != prov.SourceVersion:
				if err := store.UpdateImport(ctx, p, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
			default:
				sum.Skipped++
				continue // unchanged edition: nothing to re-enrich
			}
			if p.Placetype == "country" && p.CountryCode != "" {
				if err := store.EnrichCountry(ctx, p, prov); err != nil {
					return domain.Summary{}, err
				}
			}
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
// fails the whole transaction loudly (the glottolog mapper guarantees this ordering). On every
// insert/update the languoid's country ties are replaced. After the whole batch the handler rebuilds
// the transitive closure and the denormalized family_code in one shot (the whole import is one tx).
func LanguageSchemeHandler(newStore LanguoidStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		touched := false
		for _, rec := range records {
			l, err := languoidFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			ver, found, err := store.GetVersion(ctx, l.Code)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, l, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
				touched = true
			case ver != prov.SourceVersion:
				if err := store.UpdateImport(ctx, l, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Updated++
				touched = true
			default:
				sum.Skipped++
				continue // unchanged edition: leave country ties as-is
			}
			if err := store.ReplaceCountries(ctx, l.Code, l.Countries); err != nil {
				return domain.Summary{}, err
			}
		}
		// Rebuild the closure + family_code only when the tree actually changed (skip a full no-op import),
		// then reconcile the locale→languoid link (D-i18n) so supported UI locales point at their languoid.
		if touched {
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
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error) {
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

// ExternalOrgsHandler builds the external-organizations upsert handler (D-ExternalOrgs, M30). It resolves
// each record's kind against the external_org_kinds catalog (a record whose kind does not resolve is
// skipped — the import is resilient to mapping gaps), then keys idempotency on the Wikidata id: insert
// when absent, update only when the name changed, skip otherwise — never deletes. Imported rows are
// stamped source=imported + as_of=ImportedAt; the country (ISO alpha-2) resolves to geo_countries in SQL.
func ExternalOrgsHandler(newStore ExternalOrgStoreFactory) Handler {
	return func(ctx context.Context, tx pgx.Tx, records []domain.Record, prov domain.Provenance) (domain.Summary, error) {
		store := newStore(tx)
		var sum domain.Summary
		for _, rec := range records {
			o, err := externalOrgFields(rec)
			if err != nil {
				return domain.Summary{}, err
			}
			kindID, ok, err := store.ResolveKind(ctx, o.KindCode)
			if err != nil {
				return domain.Summary{}, err
			}
			if !ok {
				sum.Skipped++
				continue
			}
			existing, found, err := store.GetByWikidata(ctx, o.WikidataID)
			if err != nil {
				return domain.Summary{}, err
			}
			switch {
			case !found:
				if err := store.Insert(ctx, kindID, o, prov); err != nil {
					return domain.Summary{}, err
				}
				sum.Created++
			case existing != o.Name:
				if err := store.UpdateImport(ctx, kindID, o, prov); err != nil {
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
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
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
