// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// fakeLanguoidStore is an in-memory domain.LanguoidStore for testing the language-scheme handler's
// create/update/skip + country-replace + closure-rebuild control flow without a database. BulkUpsert
// mimics the merge's version-keyed semantics (insert absent, update on a different edition, skip the
// rest; CreateOnly never touches an existing row).
type fakeLanguoidStore struct {
	versions   map[string]string
	countries  map[string][]string
	rebuilds   int
	reconciles int
}

func (f *fakeLanguoidStore) BulkUpsert(_ context.Context, ls []domain.Languoid, prov domain.Provenance) (created, updated []string, _ error) {
	for _, l := range ls {
		v, ok := f.versions[l.Code]
		switch {
		case !ok:
			f.versions[l.Code] = prov.SourceVersion
			created = append(created, l.Code)
		case prov.CreateOnly:
		case v != prov.SourceVersion:
			f.versions[l.Code] = prov.SourceVersion
			updated = append(updated, l.Code)
		}
	}
	return created, updated, nil
}
func (f *fakeLanguoidStore) BulkReplaceCountries(_ context.Context, codes []string, pairCodes, pairCountries []string) error {
	if f.countries == nil {
		f.countries = map[string][]string{}
	}
	for _, c := range codes {
		f.countries[c] = nil
	}
	for i, c := range pairCodes {
		f.countries[c] = append(f.countries[c], pairCountries[i])
	}
	return nil
}
func (f *fakeLanguoidStore) RebuildClosure(_ context.Context) error { f.rebuilds++; return nil }
func (f *fakeLanguoidStore) ReconcileLocaleLanguages(_ context.Context) error {
	f.reconciles++
	return nil
}

// TestLanguageSchemeHandler exercises the source_version-keyed create/update/skip logic, the country
// replace, and the one-shot closure rebuild (only when the tree changed).
func TestLanguageSchemeHandler(t *testing.T) {
	store := &fakeLanguoidStore{versions: map[string]string{}}
	h := LanguageSchemeHandler(func(db.DBTX) domain.LanguoidStore { return store })

	batch := func() []domain.Record {
		return []domain.Record{ // parent-first: family -> language -> dialect
			{"code": "indo1319", "level": "family", "name": "Indo-European"},
			{"code": "stan1293", "level": "language", "name": "English", "parent": "indo1319",
				"iso639_3": "eng", "status": "not_endangered", "countries": []any{"GB", "us"}},
			{"code": "some1234", "level": "dialect", "name": "Some dialect", "parent": "stan1293"},
		}
	}

	sum, err := h(context.Background(), pgx.Tx(nil), batch(), domain.Provenance{Source: "glottolog", SourceVersion: "5.3"}, oneChunk)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if sum.Created != 3 || sum.Updated != 0 || sum.Skipped != 0 {
		t.Fatalf("first summary = %+v, want created=3", sum)
	}
	if store.rebuilds != 1 {
		t.Fatalf("closure rebuild = %d, want 1", store.rebuilds)
	}
	if store.reconciles != 1 {
		t.Fatalf("locale reconcile = %d, want 1", store.reconciles)
	}
	if got := store.countries["stan1293"]; len(got) != 2 || got[0] != "GB" || got[1] != "US" {
		t.Fatalf("country ties = %v, want [GB US] (upper-cased)", got)
	}

	// Same edition: pure no-op (idempotent); no closure rebuild / locale reconcile (tree unchanged).
	store.rebuilds = 0
	store.reconciles = 0
	sum, err = h(context.Background(), pgx.Tx(nil), batch(), domain.Provenance{Source: "glottolog", SourceVersion: "5.3"}, oneChunk)
	if err != nil {
		t.Fatalf("handler re-run: %v", err)
	}
	if sum.Created != 0 || sum.Updated != 0 || sum.Skipped != 3 {
		t.Fatalf("re-run summary = %+v, want all-skipped", sum)
	}
	if store.rebuilds != 0 || store.reconciles != 0 {
		t.Fatalf("closure/locale must not rebuild on a pure-skip import: rebuilds=%d reconciles=%d", store.rebuilds, store.reconciles)
	}

	// Newer edition: all updated, closure rebuilt once.
	sum, err = h(context.Background(), pgx.Tx(nil), batch(), domain.Provenance{Source: "glottolog", SourceVersion: "5.4"}, oneChunk)
	if err != nil {
		t.Fatalf("handler new edition: %v", err)
	}
	if sum.Created != 0 || sum.Updated != 3 || sum.Skipped != 0 {
		t.Fatalf("new-edition summary = %+v, want updated=3", sum)
	}
	if store.rebuilds != 1 {
		t.Fatalf("closure rebuild = %d, want 1 after update", store.rebuilds)
	}
	if store.reconciles != 1 {
		t.Fatalf("locale reconcile = %d, want 1 after update", store.reconciles)
	}
}

// TestLanguageSchemeHandler_InvalidRecord rejects records missing code/name or with a bad level.
func TestLanguageSchemeHandler_InvalidRecord(t *testing.T) {
	store := &fakeLanguoidStore{versions: map[string]string{}}
	h := LanguageSchemeHandler(func(db.DBTX) domain.LanguoidStore { return store })
	for _, rec := range []domain.Record{
		{"code": "", "level": "language", "name": "No code"},
		{"code": "stan1293", "level": "language", "name": ""},
		{"code": "stan1293", "level": "kingdom", "name": "Bad level"},
	} {
		if _, err := h(context.Background(), pgx.Tx(nil), []domain.Record{rec}, domain.Provenance{}, oneChunk); err == nil {
			t.Fatalf("expected ErrInvalidRecord for %v", rec)
		}
	}
}

// fakeScriptStore is an in-memory domain.LanguageScriptStore: a small languoid + writing-system
// resolver plus a link store, to exercise resolve→skip / create / update.
type fakeScriptStore struct {
	languoids map[string]string // iso639_3 -> languoid id
	scripts   map[string]string // iso-15924 -> ws id
	links     map[string]bool   // "lid|wid" -> is_primary
	inserts   int
	updates   int
}

func (f *fakeScriptStore) ResolveLanguoid(_ context.Context, iso string) (string, bool, error) {
	id, ok := f.languoids[iso]
	return id, ok, nil
}
func (f *fakeScriptStore) ResolveWritingSystem(_ context.Context, code string) (string, bool, error) {
	id, ok := f.scripts[code]
	return id, ok, nil
}
func (f *fakeScriptStore) GetLinkPrimary(_ context.Context, lid, wid string) (bool, bool, error) {
	p, ok := f.links[lid+"|"+wid]
	return p, ok, nil
}
func (f *fakeScriptStore) InsertLink(_ context.Context, lid, wid string, p bool, _ domain.Provenance) error {
	f.links[lid+"|"+wid] = p
	f.inserts++
	return nil
}
func (f *fakeScriptStore) UpdateLink(_ context.Context, lid, wid string, p bool, _ domain.Provenance) error {
	f.links[lid+"|"+wid] = p
	f.updates++
	return nil
}

// TestLanguageScriptsHandler exercises resolve-or-skip, create, idempotent skip, and is_primary update.
func TestLanguageScriptsHandler(t *testing.T) {
	store := &fakeScriptStore{
		languoids: map[string]string{"eng": "L-eng", "deu": "L-deu"},
		scripts:   map[string]string{"Latn": "W-Latn"},
		links:     map[string]bool{},
	}
	h := LanguageScriptsHandler(func(db.DBTX) domain.LanguageScriptStore { return store })
	prov := domain.Provenance{Source: "cldr", SourceVersion: "45"}

	recs := []domain.Record{
		{"iso639_3": "eng", "writingSystem": "Latn", "isPrimary": true},  // new -> create
		{"iso639_3": "rus", "writingSystem": "Cyrl", "isPrimary": true},  // languoid unresolved -> skip
		{"iso639_3": "deu", "writingSystem": "Runr", "isPrimary": false}, // script unseeded -> skip
	}
	sum, err := h(context.Background(), pgx.Tx(nil), recs, prov, oneChunk)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if sum.Created != 1 || sum.Updated != 0 || sum.Skipped != 2 {
		t.Fatalf("summary = %+v, want created=1 skipped=2", sum)
	}

	// Re-run the resolvable link unchanged -> skip; then flip is_primary -> update.
	sum, _ = h(context.Background(), pgx.Tx(nil), []domain.Record{{"iso639_3": "eng", "writingSystem": "Latn", "isPrimary": true}}, prov, oneChunk)
	if sum.Skipped != 1 || sum.Updated != 0 {
		t.Fatalf("idempotent re-run = %+v, want skipped=1", sum)
	}
	sum, _ = h(context.Background(), pgx.Tx(nil), []domain.Record{{"iso639_3": "eng", "writingSystem": "Latn", "isPrimary": false}}, prov, oneChunk)
	if sum.Updated != 1 {
		t.Fatalf("is_primary change = %+v, want updated=1", sum)
	}
}
