package application

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// fakeGeoStore is an in-memory domain.GeoCountryStore for testing the handler's create/update/skip
// logic without a database.
type fakeGeoStore struct {
	names    map[string]string
	inserts  int
	updates  int
	enriched int
	lastProv domain.Provenance
}

func (f *fakeGeoStore) GetName(_ context.Context, code string) (string, bool, error) {
	n, ok := f.names[code]
	return n, ok, nil
}

func (f *fakeGeoStore) Insert(_ context.Context, code, name string, prov domain.Provenance) error {
	f.names[code] = name
	f.inserts++
	f.lastProv = prov
	return nil
}

func (f *fakeGeoStore) UpdateImport(_ context.Context, code, name string, prov domain.Provenance) error {
	f.names[code] = name
	f.updates++
	f.lastProv = prov
	return nil
}

func (f *fakeGeoStore) Enrich(_ context.Context, _ string, _ domain.GeoCountryEnrichment) error {
	f.enriched++
	return nil
}

// TestGeoCountriesHandler exercises the code-keyed create/update/skip + provenance + validation paths.
func TestGeoCountriesHandler(t *testing.T) {
	store := &fakeGeoStore{names: map[string]string{"UA": "Ukraine"}}
	h := GeoCountriesHandler(func(db.DBTX) domain.GeoCountryStore { return store })
	prov := domain.Provenance{Source: "iso-3166", SourceVersion: "2024"}

	recs := []domain.Record{
		{"code": "ua", "name": "Ukraine"},           // unchanged -> skip (code lower-cased)
		{"code": "PL", "name": "Poland"},            // new -> create
		{"code": "UA", "name": "Ukraine (updated)"}, // changed -> update
	}
	sum, err := h(context.Background(), pgx.Tx(nil), recs, prov)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if sum.Created != 1 || sum.Updated != 1 || sum.Skipped != 1 {
		t.Fatalf("summary = %+v, want created=1 updated=1 skipped=1", sum)
	}
	if store.lastProv.Source != "iso-3166" {
		t.Fatalf("provenance not stamped: %+v", store.lastProv)
	}

	// Re-running the same (now-current) records is a pure no-op (idempotent).
	store.inserts, store.updates = 0, 0
	again := []domain.Record{{"code": "PL", "name": "Poland"}, {"code": "UA", "name": "Ukraine (updated)"}}
	sum2, err := h(context.Background(), pgx.Tx(nil), again, prov)
	if err != nil {
		t.Fatalf("handler re-run: %v", err)
	}
	if sum2.Created != 0 || sum2.Updated != 0 || sum2.Skipped != 2 {
		t.Fatalf("re-run summary = %+v, want all-skipped", sum2)
	}
}

// TestGeoCountriesHandler_InvalidRecord rejects a malformed record (bad code / empty name).
func TestGeoCountriesHandler_InvalidRecord(t *testing.T) {
	store := &fakeGeoStore{names: map[string]string{}}
	h := GeoCountriesHandler(func(db.DBTX) domain.GeoCountryStore { return store })
	for _, rec := range []domain.Record{
		{"code": "USA", "name": "United States"}, // 3-letter code
		{"code": "DE", "name": ""},               // empty name
		{"name": "No Code"},                      // missing code
	} {
		if _, err := h(context.Background(), pgx.Tx(nil), []domain.Record{rec}, domain.Provenance{}); err == nil {
			t.Fatalf("expected ErrInvalidRecord for %v", rec)
		}
	}
}

// fakeReligionStore is an in-memory domain.ReligionStore recording source_version per code and counting
// the create/update/skip + closure-rebuild paths (no DB needed for control-flow tests).
type fakeReligionStore struct {
	versions map[string]string
	inserts  int
	updates  int
	classes  int
	rebuilds int
}

func (f *fakeReligionStore) GetVersion(_ context.Context, code string) (string, bool, error) {
	v, ok := f.versions[code]
	return v, ok, nil
}
func (f *fakeReligionStore) Insert(_ context.Context, r domain.Religion, prov domain.Provenance) error {
	f.versions[r.Code] = prov.SourceVersion
	f.inserts++
	return nil
}
func (f *fakeReligionStore) UpdateImport(_ context.Context, r domain.Religion, prov domain.Provenance) error {
	f.versions[r.Code] = prov.SourceVersion
	f.updates++
	return nil
}
func (f *fakeReligionStore) ReplaceClassifications(_ context.Context, _ string, _ []string) error {
	f.classes++
	return nil
}
func (f *fakeReligionStore) RebuildClosure(_ context.Context) error { f.rebuilds++; return nil }

// TestReligionSchemeHandler exercises the source_version-keyed create/update/skip logic, the
// CreateOnly (boot autoseed) never-overwrite branch, and the closure rebuild only when the tree changed.
func TestReligionSchemeHandler(t *testing.T) {
	store := &fakeReligionStore{versions: map[string]string{"christianity": "2026.06"}}
	h := ReligionSchemeHandler(func(db.DBTX) domain.ReligionStore { return store })
	recs := []domain.Record{
		{"code": "christianity", "name": "Christianity", "rank": "religion"},           // exists, same version -> skip
		{"code": "calvinism", "name": "Calvinism", "rank": "tradition", "parent": "p"}, // new -> create
	}
	sum, err := h(context.Background(), pgx.Tx(nil), recs, domain.Provenance{Source: "religion-presets", SourceVersion: "2026.06"})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if sum.Created != 1 || sum.Skipped != 1 {
		t.Fatalf("summary = %+v, want created=1 skipped=1", sum)
	}
	if store.rebuilds != 1 {
		t.Fatalf("closure must rebuild once when the tree changed, got %d", store.rebuilds)
	}

	// Boot autoseed (CreateOnly) over the now-current tree: everything skipped, no rebuild.
	store.inserts, store.updates, store.rebuilds = 0, 0, 0
	sum, err = h(context.Background(), pgx.Tx(nil), recs, domain.Provenance{Source: "religion-presets", SourceVersion: "2027.01", CreateOnly: true})
	if err != nil {
		t.Fatalf("handler create-only: %v", err)
	}
	if sum.Created != 0 || sum.Updated != 0 || sum.Skipped != 2 {
		t.Fatalf("create-only summary = %+v, want all-skipped", sum)
	}
	if store.rebuilds != 0 {
		t.Fatalf("no rebuild expected on a pure no-op, got %d", store.rebuilds)
	}

	// A record missing its required rank is rejected.
	if _, err := h(context.Background(), pgx.Tx(nil), []domain.Record{{"code": "x", "name": "X"}}, domain.Provenance{}); err == nil {
		t.Fatalf("expected ErrInvalidRecord for a record with no rank")
	}
}

// fakeColorStore is an in-memory domain.ColorStore keyed on (domain, code), counting insert/update.
type fakeColorStore struct {
	colors  map[string][2]string // "domain/code" -> {name, hex}
	inserts int
	updates int
}

func (f *fakeColorStore) key(d, c string) string { return d + "/" + c }
func (f *fakeColorStore) Get(_ context.Context, d, c string) (string, string, bool, error) {
	v, ok := f.colors[f.key(d, c)]
	return v[0], v[1], ok, nil
}
func (f *fakeColorStore) Insert(_ context.Context, c domain.Color) error {
	f.colors[f.key(c.Domain, c.Code)] = [2]string{c.Name, c.Hex}
	f.inserts++
	return nil
}
func (f *fakeColorStore) UpdateImport(_ context.Context, c domain.Color) error {
	f.colors[f.key(c.Domain, c.Code)] = [2]string{c.Name, c.Hex}
	f.updates++
	return nil
}

// TestColorsHandler exercises the (domain,code)-keyed create/update/skip logic and the CreateOnly branch.
func TestColorsHandler(t *testing.T) {
	store := &fakeColorStore{colors: map[string][2]string{"country/blue": {"Blue", "#0057B7"}}}
	h := ColorsHandler(func(db.DBTX) domain.ColorStore { return store })
	recs := []domain.Record{
		{"domain": "country", "code": "blue", "name": "Blue", "hex": "#0057B7"},  // unchanged -> skip
		{"domain": "country", "code": "red", "name": "Red", "hex": "#B22234"},    // new -> create
		{"domain": "country", "code": "blue", "name": "Azure", "hex": "#0057B7"}, // name changed -> update
	}
	sum, err := h(context.Background(), pgx.Tx(nil), recs, domain.Provenance{})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if sum.Created != 1 || sum.Updated != 1 || sum.Skipped != 1 {
		t.Fatalf("summary = %+v, want created=1 updated=1 skipped=1", sum)
	}

	// CreateOnly (boot autoseed): an existing color is never overwritten even when it differs.
	store.inserts, store.updates = 0, 0
	sum, err = h(context.Background(), pgx.Tx(nil), []domain.Record{{"domain": "country", "code": "red", "name": "Crimson", "hex": "#FF0000"}},
		domain.Provenance{CreateOnly: true})
	if err != nil {
		t.Fatalf("handler create-only: %v", err)
	}
	if store.updates != 0 || sum.Skipped != 1 {
		t.Fatalf("create-only must skip an existing color: updates=%d sum=%+v", store.updates, sum)
	}

	// A record missing domain/code/name is rejected.
	if _, err := h(context.Background(), pgx.Tx(nil), []domain.Record{{"code": "x", "name": "X"}}, domain.Provenance{}); err == nil {
		t.Fatalf("expected ErrInvalidRecord for a color with no domain")
	}
}

// fakeGeoPlaceStore is an in-memory domain.GeoPlaceStore that records source_version per wof_id and
// counts the create/update/enrich paths (no DB needed for the handler's control flow).
type fakeGeoPlaceStore struct {
	versions map[int64]string
	inserts  int
	updates  int
	enrich   int
}

func (f *fakeGeoPlaceStore) GetVersion(_ context.Context, wofID int64) (string, bool, error) {
	v, ok := f.versions[wofID]
	return v, ok, nil
}
func (f *fakeGeoPlaceStore) Insert(_ context.Context, p domain.GeoPlace, prov domain.Provenance) error {
	f.versions[p.WofID] = prov.SourceVersion
	f.inserts++
	return nil
}
func (f *fakeGeoPlaceStore) UpdateImport(_ context.Context, p domain.GeoPlace, prov domain.Provenance) error {
	f.versions[p.WofID] = prov.SourceVersion
	f.updates++
	return nil
}
func (f *fakeGeoPlaceStore) EnrichCountry(_ context.Context, _ domain.GeoPlace, _ domain.Provenance) error {
	f.enrich++
	return nil
}

// TestGeoPlacesHandler exercises the source_version-keyed create/update/skip logic plus the
// country-enrichment side effect, across three editions of the same parent-first batch.
func TestGeoPlacesHandler(t *testing.T) {
	store := &fakeGeoPlaceStore{versions: map[int64]string{}}
	h := GeoPlacesHandler(func(db.DBTX) domain.GeoPlaceStore { return store })

	batch := func() []domain.Record {
		return []domain.Record{ // parent-first: country -> region -> locality
			{"wofId": float64(85633267), "placetype": "country", "name": "Ukraine", "countryCode": "ua",
				"isoA3": "ukr", "geometry": map[string]any{"type": "Point", "coordinates": []any{31.0, 49.0}}},
			{"wofId": float64(85632563), "placetype": "region", "name": "Kyiv City", "countryCode": "UA",
				"parentId": float64(85633267), "population": float64(2900000)},
			{"wofId": float64(101751957), "placetype": "locality", "name": "Kyiv", "countryCode": "UA",
				"parentId": float64(85632563)},
		}
	}

	// First edition: all three inserted, the country row enriched once.
	sum, err := h(context.Background(), pgx.Tx(nil), batch(), domain.Provenance{Source: "whosonfirst", SourceVersion: "2026.01"})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if sum.Created != 3 || sum.Updated != 0 || sum.Skipped != 0 {
		t.Fatalf("first summary = %+v, want created=3", sum)
	}
	if store.enrich != 1 {
		t.Fatalf("country enrich = %d, want 1", store.enrich)
	}

	// Same edition: pure no-op (idempotent); no re-enrich.
	sum, err = h(context.Background(), pgx.Tx(nil), batch(), domain.Provenance{Source: "whosonfirst", SourceVersion: "2026.01"})
	if err != nil {
		t.Fatalf("handler re-run: %v", err)
	}
	if sum.Created != 0 || sum.Updated != 0 || sum.Skipped != 3 {
		t.Fatalf("re-run summary = %+v, want all-skipped", sum)
	}
	if store.enrich != 1 {
		t.Fatalf("enrich must not run on skip: %d", store.enrich)
	}

	// Newer edition: all updated, the country re-enriched.
	sum, err = h(context.Background(), pgx.Tx(nil), batch(), domain.Provenance{Source: "whosonfirst", SourceVersion: "2026.06"})
	if err != nil {
		t.Fatalf("handler new edition: %v", err)
	}
	if sum.Created != 0 || sum.Updated != 3 || sum.Skipped != 0 {
		t.Fatalf("new-edition summary = %+v, want updated=3", sum)
	}
	if store.enrich != 2 {
		t.Fatalf("country enrich = %d, want 2 after update", store.enrich)
	}
}

// TestGeoPlacesHandler_InvalidRecord rejects records missing wofId/name or with an unknown placetype.
func TestGeoPlacesHandler_InvalidRecord(t *testing.T) {
	store := &fakeGeoPlaceStore{versions: map[int64]string{}}
	h := GeoPlacesHandler(func(db.DBTX) domain.GeoPlaceStore { return store })
	for _, rec := range []domain.Record{
		{"placetype": "region", "name": "No Id"},                          // missing wofId
		{"wofId": float64(1), "placetype": "region"},                      // missing name
		{"wofId": float64(1), "placetype": "neighbourhood", "name": "Nb"}, // unsupported placetype
	} {
		if _, err := h(context.Background(), pgx.Tx(nil), []domain.Record{rec}, domain.Provenance{}); err == nil {
			t.Fatalf("expected ErrInvalidRecord for %v", rec)
		}
	}
}
