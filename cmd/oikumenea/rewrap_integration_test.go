//go:build integration

// Integration proof for the R-22 key-rotation rewrap CLI against a real Postgres. It exercises the
// exact production loop (rewrapTable / reindexTable) over a temporary envelope-shaped table — the real
// table *names* are guaranteed complete separately by TestRewrapTablesMatchSchema (unit), so this test
// focuses on the DB behavior: rewrap flips every DEK onto the active KEK, values still decrypt (and
// decrypt under a B-only provider, proving the row truly moved off A), a re-run is a no-op / resumes,
// and the blind-index reindex pass recomputes under a new key.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./cmd/oikumenea/ -run TestRewrapCLI
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
)

const rewrapDefaultDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func rewrapTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = rewrapDefaultDSN
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test db unreachable (%v); set OIKUMENEA_TEST_DSN", err)
	}
	return pool
}

func newCipher(t *testing.T, active []byte, previous [][]byte, blind string) *crypto.Cipher {
	t.Helper()
	prov, err := crypto.NewLocalDevProviderWithPrevious(active, previous)
	if err != nil {
		t.Fatal(err)
	}
	c, err := crypto.NewCipher(prov, []byte(blind), 0)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func rewrapCount(t *testing.T, pool *pgxpool.Pool, where string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM oikumenea.rewrap_e2e_tmp WHERE "+where, args...).Scan(&n); err != nil {
		t.Fatalf("count(%s): %v", where, err)
	}
	return n
}

func TestRewrapCLI_Integration(t *testing.T) {
	ctx := context.Background()
	pool := rewrapTestPool(t)
	defer pool.Close()

	// A throwaway table with the canonical envelope shape. Dropped on exit.
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS oikumenea.rewrap_e2e_tmp;
		CREATE TABLE oikumenea.rewrap_e2e_tmp (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			value_ciphertext bytea, wrapped_dek bytea, key_ref text, value_blind_index bytea)`); err != nil {
		t.Fatalf("create temp table: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP TABLE IF EXISTS oikumenea.rewrap_e2e_tmp") })

	tmp := encTable{
		name: "oikumenea.rewrap_e2e_tmp", ciphertext: "value_ciphertext",
		wrappedDEK: "wrapped_dek", keyRef: "key_ref", blindIndex: "value_blind_index",
	}

	keyA := bytes.Repeat([]byte{0xA1}, 32)
	keyB := bytes.Repeat([]byte{0xB2}, 32)
	cipherA := newCipher(t, keyA, nil, "blind-key-1")
	keyRefA := cipherA.ActiveKeyRef()

	// Seed under A. plaintexts[value] lets us verify decryption later.
	plaintexts := map[string][]byte{}
	seed := func(n int) {
		for i := 0; i < n; i++ {
			pt := []byte(fmt.Sprintf("value-%d-%s", i, keyRefA))
			s, err := cipherA.Seal(ctx, pt)
			if err != nil {
				t.Fatal(err)
			}
			var id string
			if err := pool.QueryRow(ctx,
				`INSERT INTO oikumenea.rewrap_e2e_tmp (value_ciphertext, wrapped_dek, key_ref, value_blind_index)
				 VALUES ($1,$2,$3,$4) RETURNING id::text`,
				s.Ciphertext, s.WrappedDEK, s.KeyRef, cipherA.BlindIndex(pt)).Scan(&id); err != nil {
				t.Fatalf("seed insert: %v", err)
			}
			plaintexts[id] = pt
		}
	}
	seed(300)

	// Rotate to B (A retained as previous). rewrap in small batches.
	cipherB := newCipher(t, keyB, [][]byte{keyA}, "blind-key-1")
	keyRefB := cipherB.ActiveKeyRef()
	if keyRefB == keyRefA {
		t.Fatal("precondition: active key_ref must change after rotation")
	}

	n, err := rewrapTable(ctx, pool, cipherB, tmp, keyRefB, 100)
	if err != nil {
		t.Fatalf("rewrap: %v", err)
	}
	if n != 300 {
		t.Fatalf("rewrap count = %d, want 300", n)
	}
	if left := rewrapCount(t, pool, "key_ref <> $1", keyRefB); left != 0 {
		t.Fatalf("%d rows remain off the active key_ref after rewrap", left)
	}
	if onB := rewrapCount(t, pool, "key_ref = $1", keyRefB); onB != 300 {
		t.Fatalf("only %d/300 rows on the active key_ref", onB)
	}

	// A B-ONLY provider must decrypt every row — proving the DEKs genuinely moved off A (not just relabeled).
	cipherBonly := newCipher(t, keyB, nil, "blind-key-1")
	rows, err := pool.Query(ctx, `SELECT id::text, value_ciphertext, wrapped_dek, key_ref FROM oikumenea.rewrap_e2e_tmp`)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for rows.Next() {
		var id, kr string
		var ct, wd []byte
		if err := rows.Scan(&id, &ct, &wd, &kr); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		got, err := cipherBonly.Open(ctx, crypto.Sealed{Ciphertext: ct, WrappedDEK: wd, KeyRef: kr})
		if err != nil || !bytes.Equal(got, plaintexts[id]) {
			rows.Close()
			t.Fatalf("row %s: B-only open failed: got %q err %v", id, got, err)
		}
		checked++
	}
	rows.Close()
	if checked != 300 {
		t.Fatalf("decrypt-checked %d rows, want 300", checked)
	}

	// Re-run rewrap: idempotent no-op (this is also the resume story — committed batches stay done).
	if n, err := rewrapTable(ctx, pool, cipherB, tmp, keyRefB, 100); err != nil || n != 0 {
		t.Fatalf("re-run rewrap = %d, err %v; want 0 (idempotent)", n, err)
	}

	// Resume proof: rows a crashed run hadn't reached (fresh A rows) are picked up on the next run.
	seed(100)
	if n, err := rewrapTable(ctx, pool, cipherB, tmp, keyRefB, 100); err != nil || n != 100 {
		t.Fatalf("resume rewrap = %d, err %v; want 100 (only the not-yet-migrated rows)", n, err)
	}
	if left := rewrapCount(t, pool, "key_ref <> $1", keyRefB); left != 0 {
		t.Fatalf("%d rows still off active key_ref after resume", left)
	}

	// Blind-index reindex under a NEW blind key. Capture one row's old index first.
	var sampleID string
	var oldBlind []byte
	if err := pool.QueryRow(ctx, `SELECT id::text, value_blind_index FROM oikumenea.rewrap_e2e_tmp LIMIT 1`).Scan(&sampleID, &oldBlind); err != nil {
		t.Fatal(err)
	}
	cipherReindex := newCipher(t, keyB, nil, "blind-key-2-rotated")
	rn, err := reindexTable(ctx, pool, cipherReindex, tmp, 100)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if rn != 400 {
		t.Fatalf("reindex count = %d, want 400", rn)
	}
	var newBlind []byte
	if err := pool.QueryRow(ctx, `SELECT value_blind_index FROM oikumenea.rewrap_e2e_tmp WHERE id = $1::uuid`, sampleID).Scan(&newBlind); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(newBlind, oldBlind) {
		t.Fatal("reindex did not change the blind index")
	}
	if want := cipherReindex.BlindIndex(plaintexts[sampleID]); !bytes.Equal(newBlind, want) {
		t.Fatal("reindexed blind index does not match recompute under the new key")
	}
}
