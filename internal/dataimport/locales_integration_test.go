// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for the `locales` import object-type (D-DataPacks + D-i18n, M54) — the path a LOCALE
// PACK uses to add a supported locale before its translation overlays. Drives the real registered
// handler + LocaleRepo against a migrated Postgres: a new code is added create-if-absent, a re-import is
// a no-op, and an existing operator-managed locale's flags are never touched.
package dataimport_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olehmushka/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/adapters"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/application"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/domain"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

const localesTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func localesPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = localesTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newLocalesImportService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	svc := application.NewService(pool, audit)
	svc.Register(domain.ObjectTypeLocales, application.LocalesHandler(
		func(conn pdb.DBTX) domain.LocaleStore { return adapters.NewLocaleRepo(conn) },
	))
	return svc
}

func TestLocalesImportCreateIfAbsent(t *testing.T) {
	ctx := context.Background()
	pool := localesPool(t)
	svc := newLocalesImportService(t, pool)

	const code = "zul" // isiZulu — not among the seeded ukr/eng/spa/por, so this run must create it
	// Clean slate + cleanup so the test is repeatable and leaves no residue.
	_, _ = pool.Exec(ctx, `DELETE FROM oikumenea.i18n_locales WHERE code = $1`, code)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM oikumenea.i18n_locales WHERE code = $1`, code)
	})

	env := application.Envelope{
		ObjectType:    domain.ObjectTypeLocales,
		Source:        "test",
		SourceVersion: "1",
		CreateOnly:    true,
		Records:       []domain.Record{{"code": code, "name": "isiZulu"}},
	}

	// 1) First import creates the locale.
	sum, err := svc.Import(ctx, domain.ObjectTypeLocales, env)
	if err != nil {
		t.Fatalf("import locales: %v", err)
	}
	if sum.Created != 1 {
		t.Fatalf("created = %d, want 1", sum.Created)
	}
	var name string
	var enabled, isDefault bool
	if err := pool.QueryRow(ctx,
		`SELECT name, enabled, is_default FROM oikumenea.i18n_locales WHERE code = $1`, code,
	).Scan(&name, &enabled, &isDefault); err != nil {
		t.Fatalf("read seeded locale: %v", err)
	}
	if name != "isiZulu" || !enabled || isDefault {
		t.Fatalf("locale row = {name:%q enabled:%v is_default:%v}, want {isiZulu true false}", name, enabled, isDefault)
	}

	// 2) An operator disables it — a pack must never re-enable or clobber that.
	if _, err := pool.Exec(ctx, `UPDATE oikumenea.i18n_locales SET enabled = false, name = 'Zulu (operator)' WHERE code = $1`, code); err != nil {
		t.Fatal(err)
	}

	// 3) Re-import: create-if-absent → skipped, and the operator's edits survive untouched.
	sum2, err := svc.Import(ctx, domain.ObjectTypeLocales, env)
	if err != nil {
		t.Fatalf("re-import locales: %v", err)
	}
	if sum2.Created != 0 || sum2.Skipped != 1 {
		t.Fatalf("re-import summary = {created:%d skipped:%d}, want {0 1}", sum2.Created, sum2.Skipped)
	}
	if err := pool.QueryRow(ctx,
		`SELECT name, enabled FROM oikumenea.i18n_locales WHERE code = $1`, code,
	).Scan(&name, &enabled); err != nil {
		t.Fatal(err)
	}
	if name != "Zulu (operator)" || enabled {
		t.Fatalf("operator edits clobbered: name=%q enabled=%v (want 'Zulu (operator)' false)", name, enabled)
	}
}
