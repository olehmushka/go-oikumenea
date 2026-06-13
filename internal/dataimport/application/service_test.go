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

// TestGeoCountriesHandler exercises the code-keyed create/update/skip + provenance + validation paths.
func TestGeoCountriesHandler(t *testing.T) {
	store := &fakeGeoStore{names: map[string]string{"UA": "Ukraine"}}
	h := GeoCountriesHandler(func(db.DBTX) domain.GeoCountryStore { return store })
	prov := domain.Provenance{Source: "iso-3166", SourceVersion: "2024"}

	recs := []domain.Record{
		{"code": "ua", "name": "Ukraine"},          // unchanged -> skip (code lower-cased)
		{"code": "PL", "name": "Poland"},           // new -> create
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
