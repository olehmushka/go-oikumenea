// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the connector plane against a real Postgres (M53 exit criteria,
// D-ConnectorPlane / D-Audit). They exercise self-registration bound to the calling principal, the
// idempotent re-registration that retires declared-away sources, the anti-impersonation conflict, the
// idempotent run report keyed on (source, externalRunId), last-seen bookkeeping, and the operator
// reads. The transport's principal-from-context binding is exercised by the live e2e in verification;
// here the application layer is driven directly with an explicit principal.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/connector/...
package connector_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olehmushka/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/olehmushka/go-oikumenea/internal/connector/adapters"
	"github.com/olehmushka/go-oikumenea/internal/connector/application"
	"github.com/olehmushka/go-oikumenea/internal/connector/domain"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	}, audit)
}

// seedPrincipal inserts a service principal and returns its RID. connector_connectors.principal_id
// FKs to it, so a registration needs a real one. Unique per test via the subject.
func seedPrincipal(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO oikumenea.account_service_principals (code, name, issuer, subject)
		VALUES ($1, $1, 'urn:test', $1)
		RETURNING id`, code).Scan(&id)
	if err != nil {
		t.Fatalf("seed principal %s: %v", code, err)
	}
	t.Cleanup(func() {
		// connectors RESTRICT the principal, so clear them first.
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM oikumenea.connector_connectors WHERE principal_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM oikumenea.account_service_principals WHERE id = $1`, id)
	})
	return id
}

func TestRegisterBindsToPrincipalAndReplacesSources(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	pid := seedPrincipal(t, pool, "conn-reg-"+ts())

	// Initial registration with two sources.
	c, srcs, err := svc.Register(ctx, domain.RegistrationInput{
		PrincipalID: pid,
		Code:        "hermenea-test",
		Name:        "hermenea (test)",
		Sources: []domain.SourceDeclaration{
			{Code: "geo-countries", Name: "Countries", ObjectType: "geo-countries", Enabled: true},
			{Code: "languages", Name: "Languages", ObjectType: "language-scheme", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if c.PrincipalID != pid {
		t.Fatalf("connector bound to %q, want caller principal %q", c.PrincipalID, pid)
	}
	if len(srcs) != 2 {
		t.Fatalf("got %d sources, want 2", len(srcs))
	}

	// Re-register with one source dropped and one added: the declared set is authoritative, so the
	// dropped source is retired and only the new set remains.
	c2, srcs2, err := svc.Register(ctx, domain.RegistrationInput{
		PrincipalID: pid,
		Code:        "hermenea-test",
		Name:        "hermenea (test, renamed)",
		Sources: []domain.SourceDeclaration{
			{Code: "geo-countries", Name: "Countries", ObjectType: "geo-countries", Enabled: true},
			{Code: "religions", Name: "Religions", ObjectType: "religion-scheme", Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if c2.ID != c.ID {
		t.Fatalf("re-registration created a new row (%s != %s); must update in place", c2.ID, c.ID)
	}
	if c2.Name != "hermenea (test, renamed)" {
		t.Fatalf("name not updated: %q", c2.Name)
	}
	got := map[string]bool{}
	for _, s := range srcs2 {
		got[s.Code] = true
	}
	if !got["geo-countries"] || !got["religions"] || got["languages"] || len(srcs2) != 2 {
		t.Fatalf("source set not reconciled: %v", srcs2)
	}
}

func TestRegisterConflictOnForeignCode(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	pidA := seedPrincipal(t, pool, "conn-a-"+ts())
	pidB := seedPrincipal(t, pool, "conn-b-"+ts())

	if _, _, err := svc.Register(ctx, domain.RegistrationInput{PrincipalID: pidA, Code: "shared", Name: "A"}); err != nil {
		t.Fatalf("register A: %v", err)
	}
	// A different principal claiming the same code must be rejected — the anti-impersonation guard.
	_, _, err := svc.Register(ctx, domain.RegistrationInput{PrincipalID: pidB, Code: "shared", Name: "B"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("foreign-code register = %v; want ErrConflict", err)
	}
}

func TestReportIsIdempotentAndTouchesLastSeen(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	pid := seedPrincipal(t, pool, "conn-run-"+ts())

	reg, _, err := svc.Register(ctx, domain.RegistrationInput{
		PrincipalID: pid, Code: "reporter", Name: "Reporter",
		Sources: []domain.SourceDeclaration{{Code: "src1", Name: "Src 1", Enabled: true}},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	before := reg.LastSeenAt
	start := time.Now().Add(-time.Minute)
	// Open a run.
	open, err := svc.Report(ctx, domain.ReportInput{
		PrincipalID: pid, SourceCode: "src1", ExternalRunID: "run-42",
		State: domain.RunRunning, StartedAt: &start,
	})
	if err != nil {
		t.Fatalf("open run: %v", err)
	}
	// Close the SAME run: idempotent on (source, externalRunId) → same row, now terminal.
	fin := time.Now()
	closed, err := svc.Report(ctx, domain.ReportInput{
		PrincipalID: pid, SourceCode: "src1", ExternalRunID: "run-42",
		State: domain.RunSucceeded, Created: 10, Updated: 2, Skipped: 100,
		StartedAt: &start, FinishedAt: &fin,
	})
	if err != nil {
		t.Fatalf("close run: %v", err)
	}
	if closed.ID != open.ID {
		t.Fatalf("close created a new run (%s != %s); must update the open one", closed.ID, open.ID)
	}
	if closed.State != domain.RunSucceeded || closed.Created != 10 || closed.Skipped != 100 {
		t.Fatalf("run not updated: %+v", closed)
	}

	runs, err := svc.ListRuns(ctx, open.SourceID, "", 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1 (idempotent report must not duplicate)", len(runs))
	}

	// last_seen_at must advance on a report (liveness bookkeeping).
	c, err := svc.GetConnector(ctx, reg.ID)
	if err != nil {
		t.Fatalf("get connector: %v", err)
	}
	if c.LastSeenAt == nil {
		t.Fatal("last_seen_at not set after a report")
	}
	if before != nil && !c.LastSeenAt.After(*before) {
		t.Fatalf("last_seen_at did not advance: %v -> %v", *before, *c.LastSeenAt)
	}
}

func TestReportUnknownSourceIsNotFound(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	pid := seedPrincipal(t, pool, "conn-nosrc-"+ts())
	if _, _, err := svc.Register(ctx, domain.RegistrationInput{PrincipalID: pid, Code: "nosrc", Name: "NoSrc"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	fin := time.Now()
	_, err := svc.Report(ctx, domain.ReportInput{
		PrincipalID: pid, SourceCode: "ghost", State: domain.RunSucceeded, FinishedAt: &fin,
	})
	if !errors.Is(err, domain.ErrSourceNotFound) {
		t.Fatalf("report to unknown source = %v; want ErrSourceNotFound", err)
	}
}

func ts() string { return time.Now().Format("150405.000000") }
