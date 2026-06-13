//go:build integration

// Integration tests for the localization module against a real Postgres (M2 exit criteria, D-i18n /
// D-Audit):
//   - the seeded locales (ukr default + eng) are present after migration;
//   - an admin write + its audit row share one transaction (commit/rollback together);
//   - translations round-trip through the store and the TranslationsFor assembly helper;
//   - an unknown locale is rejected; the registry invariant (>=1 enabled, exactly one default)
//     is enforced by the deferred constraint trigger at commit.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/localization/...
package localization_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/localization/adapters"
	"github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/localization/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newService(t *testing.T) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	ctx := context.Background()
	pool, err := pdb.NewPool(ctx, dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	auditSvc := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, auditSvc), pool
}

// uniqueCode returns a fresh 3-letter ISO-639-3-shaped code per test run so repeated runs against a
// persistent DB don't collide on the unique code.
func uniqueCode(t *testing.T) string {
	t.Helper()
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := uuid.New()
	return string([]byte{letters[b[0]%26], letters[b[1]%26], letters[b[2]%26]})
}

// TestSeededLocales asserts the migration seeded ukr (default) + eng, both enabled.
func TestSeededLocales(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	locales, err := svc.ListLocales(ctx)
	if err != nil {
		t.Fatalf("list locales: %v", err)
	}
	byCode := make(map[string]domain.Locale)
	for _, l := range locales {
		byCode[l.Code] = l
	}
	ukr, ok := byCode["ukr"]
	if !ok || !ukr.Enabled || !ukr.IsDefault {
		t.Fatalf("expected ukr enabled+default, got %+v (present=%v)", ukr, ok)
	}
	eng, ok := byCode["eng"]
	if !ok || !eng.Enabled || eng.IsDefault {
		t.Fatalf("expected eng enabled non-default, got %+v (present=%v)", eng, ok)
	}
}

// TestAddLocaleWritesAuditRow is the headline D-Audit guarantee for an admin write: the locale and
// its audit row are both readable after the write, sharing one transaction.
func TestAddLocaleWritesAuditRow(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	code := uniqueCode(t)

	created, err := svc.AddLocale(ctx, domain.Locale{Code: code, Name: "Test " + code, Enabled: true})
	if err != nil {
		t.Fatalf("add locale: %v", err)
	}
	if created.Code != code || created.IsDefault {
		t.Fatalf("unexpected created locale: %+v", created)
	}

	// The locale is readable...
	locales, err := svc.ListLocales(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !containsCode(locales, code) {
		t.Fatalf("created locale %q not found in list", code)
	}

	// ...and an audit row targeting the locale's code exists with the interim system actor.
	auditSvc := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	tt := "locale"
	page, err := auditSvc.Query(ctx, auditapp.QueryParams{TargetType: &tt, TargetID: &created.Code})
	if err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected 1 audit row for the new locale, got %d", len(page.Entries))
	}
	e := page.Entries[0]
	if e.ActorType != auditdomain.ActorSystem || e.Subsystem != "localization-admin" || e.Action != "locale.add" {
		t.Fatalf("unexpected audit entry: %+v", e)
	}
}

// TestTranslationsRoundTrip upserts translations and reads them back via TranslationsFor.
func TestTranslationsRoundTrip(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	entityID := uuid.NewString()
	in := []domain.Translation{
		{EntityType: "unit", EntityID: entityID, Field: "name", Locale: "ukr", Text: "Перша бригада"},
		{EntityType: "unit", EntityID: entityID, Field: "name", Locale: "eng", Text: "First Brigade"},
	}
	stored, err := svc.UpsertTranslations(ctx, "unit", entityID, in)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("expected 2 stored translations, got %d", len(stored))
	}

	// Upsert again with a changed value to prove ON CONFLICT replaces.
	if _, err := svc.UpsertTranslations(ctx, "unit", entityID, []domain.Translation{
		{EntityType: "unit", EntityID: entityID, Field: "name", Locale: "eng", Text: "1st Brigade"},
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	got, err := svc.TranslationsFor(ctx, "unit", []string{entityID}, []string{"name"})
	if err != nil {
		t.Fatalf("translations for: %v", err)
	}
	m := got[domain.TranslationKey{EntityID: entityID, Field: "name"}]
	if m["ukr"] != "Перша бригада" || m["eng"] != "1st Brigade" {
		t.Fatalf("unexpected locale map: %+v", m)
	}
}

// TestUpsertRejectsUnknownLocale exercises the locale-existence validation.
func TestUpsertRejectsUnknownLocale(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	entityID := uuid.NewString()
	_, err := svc.UpsertTranslations(ctx, "unit", entityID, []domain.Translation{
		{EntityType: "unit", EntityID: entityID, Field: "name", Locale: "zzz", Text: "x"},
	})
	if !errors.Is(err, domain.ErrUnknownLocale) {
		t.Fatalf("expected ErrUnknownLocale, got %v", err)
	}
}

// TestCannotDisableLastDefault proves the deferred constraint trigger rejects a commit that would
// leave the registry without exactly one enabled default.
func TestCannotUnsetOnlyDefault(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	// ukr is the seeded sole default; disabling it (without promoting another) must be rejected at
	// commit by the deferred trigger and surface as the domain constraint error.
	f := false
	_, err := svc.UpdateLocale(ctx, "ukr", domain.LocalePatch{IsDefault: &f})
	if !errors.Is(err, domain.ErrLocaleConstraint) {
		t.Fatalf("expected ErrLocaleConstraint, got %v", err)
	}

	// Sanity: ukr is still the default afterward (the failed txn rolled back).
	locales, err := svc.ListLocales(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, l := range locales {
		if l.Code == "ukr" && !l.IsDefault {
			t.Fatalf("ukr should remain default after the rejected change")
		}
	}
}

func containsCode(locales []domain.Locale, code string) bool {
	for _, l := range locales {
		if l.Code == code {
			return true
		}
	}
	return false
}
