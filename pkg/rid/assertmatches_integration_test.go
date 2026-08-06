// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for the RID registry drift guard (review-2026-09 R-28). AssertMatches is wired into
// oikumenea boot (cmd/oikumenea/main.go) so the Go registry (pkg/rid) can never silently diverge from
// the migration-seeded platform_rid_services / platform_rid_types tables. This proves it passes against
// the real seed and fails on a tampered row — the boot behaviour, without standing up the full server.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./pkg/rid/...
package rid_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/pkg/rid"
)

func TestAssertMatchesAgainstSeed(t *testing.T) {
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	defer pool.Close()

	// 1. The committed Go registry matches the seeded SQL registry — what boot asserts.
	if err := rid.AssertMatches(ctx, pool); err != nil {
		t.Fatalf("AssertMatches should pass against the real seed, got: %v", err)
	}

	// 2. Tamper one seeded type name inside a rolled-back tx → AssertMatches must catch the drift.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	// Rename the person object type (6,1,1) → AssertMatches must reject the mismatch.
	if _, err := tx.Exec(ctx,
		`UPDATE oikumenea.platform_rid_types SET type_name = 'tampered' WHERE service_code = 6 AND kind = 1 AND type_code = 1`); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	if err := rid.AssertMatches(ctx, tx); err == nil {
		t.Fatal("AssertMatches should FAIL against a tampered platform_rid_types, but returned nil")
	}
}
