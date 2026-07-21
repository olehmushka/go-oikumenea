package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/palantir/witchcraft-go-logging/wlog"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

// ---------------------------------------------------------------- key-rotation rewrap CLI (R-22)

// encTable describes one envelope-encrypted table for the rewrap tool (review R-22 / D-CryptoProvider):
// the id PK plus the four envelope columns. Column names vary (unprefixed value_* vs. iban_/pan_/party_/
// leaning_ prefixes), so each is spelled out. This registry is code-defined (never user input), so it is
// safe to interpolate into SQL.
type encTable struct {
	name       string // schema-qualified table
	ciphertext string
	wrappedDEK string
	keyRef     string
	blindIndex string
}

// encTables is the full set of envelope-encrypted tables (D-DataScope). Keep in sync with the
// `*_wrapped_dek` columns in migrations/ — TestRewrapTablesMatchSchema guards it.
var encTables = []encTable{
	{"oikumenea.document_personal_codes", "value_ciphertext", "wrapped_dek", "key_ref", "value_blind_index"},
	{"oikumenea.person_ethnicities", "value_ciphertext", "wrapped_dek", "key_ref", "value_blind_index"},
	{"oikumenea.religion_affiliations", "value_ciphertext", "wrapped_dek", "key_ref", "value_blind_index"},
	{"oikumenea.finance_accounts", "iban_ciphertext", "iban_wrapped_dek", "key_ref", "iban_blind_index"},
	{"oikumenea.finance_cards", "pan_ciphertext", "pan_wrapped_dek", "key_ref", "pan_blind_index"},
	{"oikumenea.person_party_memberships", "party_ciphertext", "party_wrapped_dek", "party_key_ref", "party_blind_index"},
	{"oikumenea.person_political_leaning", "leaning_ciphertext", "leaning_wrapped_dek", "leaning_key_ref", "leaning_blind_index"},
	{"oikumenea.person_health_records", "detail_ciphertext", "detail_wrapped_dek", "detail_key_ref", "detail_blind_index"},
}

// runRewrapCLI re-wraps every envelope-encrypted DEK under the ACTIVE KEK (review R-22, key rotation):
// it unwraps each wrapped_dek with whichever KEK produced it (active or a configured previous KEK) and
// re-wraps it with the active KEK, flipping key_ref — the value ciphertext is never touched. Operator-
// host-gated like the seed/admin CLIs (possession of the operator config is the authorization). It is
// batched and resumable: each pass only sees rows whose key_ref is not yet the active one, so a re-run
// (or a kill -9 mid-run) simply continues. With --reindex-blind-index it also runs a second pass that
// decrypts each value and recomputes its blind index under the (new) blind-index key.
func runRewrapCLI(args []string) int {
	const cmd = "rewrap"
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var configPath string
	var dryRun, reindex bool
	var batch int
	fs.StringVar(&configPath, "config", "var/conf/install.yml", "path to the install config")
	fs.BoolVar(&dryRun, "dry-run", false, "report the key_ref census per table; make no changes")
	fs.BoolVar(&reindex, "reindex-blind-index", false, "also recompute every blind index under the active blind-index key (heavier: decrypts each value)")
	fs.IntVar(&batch, "batch", 500, "rows per transaction")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if batch < 1 {
		batch = 1
	}

	install, err := loadInstall(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: load install config: %v\n", cmd, err)
		return 1
	}
	ctx := svc1log.WithLogger(context.Background(), svc1log.New(os.Stderr, wlog.InfoLevel))
	pool, err := db.NewPool(ctx, install.Postgres.DSN, install.Environment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: connect database: %v\n", cmd, err)
		return 1
	}
	defer pool.Close()

	rev, err := db.ReadSchemaRevision(ctx, pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: read schema version: %v\n", cmd, err)
		return 1
	}
	if rev != db.ExpectedSchemaRevision {
		fmt.Fprintf(os.Stderr, "%s: schema revision %q != expected %q; run migrations first\n", cmd, rev, db.ExpectedSchemaRevision)
		return 1
	}

	cipher, err := buildCipher(install)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: build cipher: %v\n", cmd, err)
		return 1
	}
	active := cipher.ActiveKeyRef()

	if dryRun {
		fmt.Fprintf(os.Stdout, "%s: DRY RUN — active key_ref = %s\n", cmd, active)
		for _, t := range encTables {
			if err := rewrapCensus(ctx, pool, t, active); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s census: %v\n", cmd, t.name, err)
				return 1
			}
		}
		return 0
	}

	fmt.Fprintf(os.Stdout, "%s: re-wrapping DEKs onto active key_ref = %s\n", cmd, active)
	for _, t := range encTables {
		n, err := rewrapTable(ctx, pool, cipher, t, active, batch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s rewrap: %v\n", cmd, t.name, err)
			return 1
		}
		fmt.Fprintf(os.Stdout, "%s: %-38s rewrapped=%d\n", cmd, t.name, n)
	}

	if reindex {
		fmt.Fprintf(os.Stdout, "%s: recomputing blind indexes under the active blind-index key\n", cmd)
		for _, t := range encTables {
			n, err := reindexTable(ctx, pool, cipher, t, batch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s reindex: %v\n", cmd, t.name, err)
				return 1
			}
			fmt.Fprintf(os.Stdout, "%s: %-38s reindexed=%d\n", cmd, t.name, n)
		}
	}
	fmt.Fprintf(os.Stdout, "%s: done\n", cmd)
	return 0
}

// rewrapCensus prints, per stored key_ref, how many un-erased rows carry it — so an operator can see
// how much a rotation has left to do (rows not on the active key_ref).
func rewrapCensus(ctx context.Context, pool *pgxpool.Pool, t encTable, active string) error {
	q := fmt.Sprintf(`SELECT COALESCE(%s,'(null)'), count(*) FROM %s WHERE %s IS NOT NULL GROUP BY 1 ORDER BY 1`,
		t.keyRef, t.name, t.wrappedDEK)
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	any := false
	for rows.Next() {
		var ref string
		var n int64
		if err := rows.Scan(&ref, &n); err != nil {
			return err
		}
		mark := ""
		if ref == active {
			mark = "  <- active"
		}
		fmt.Fprintf(os.Stdout, "  %-38s %-24s %8d%s\n", t.name, ref, n, mark)
		any = true
	}
	if !any {
		fmt.Fprintf(os.Stdout, "  %-38s (no encrypted rows)\n", t.name)
	}
	return rows.Err()
}

// rewrapTable re-wraps every row of one table whose key_ref is not the active one, in batches. Each
// batch is a transaction; because the selection predicate excludes rows already on the active key_ref,
// the loop naturally drains and a re-run resumes.
func rewrapTable(ctx context.Context, pool *pgxpool.Pool, cipher *crypto.Cipher, t encTable, active string, batch int) (int, error) {
	sel := fmt.Sprintf(`SELECT id::text, %s FROM %s WHERE %s IS NOT NULL AND (%s IS NULL OR %s <> $1) ORDER BY id LIMIT $2`,
		t.wrappedDEK, t.name, t.wrappedDEK, t.keyRef, t.keyRef)
	upd := fmt.Sprintf(`UPDATE %s SET %s = $1, %s = $2 WHERE id = $3::uuid`, t.name, t.wrappedDEK, t.keyRef)

	total := 0
	for {
		type row struct {
			id      string
			wrapped []byte
		}
		var batchRows []row
		rows, err := pool.Query(ctx, sel, active, batch)
		if err != nil {
			return total, err
		}
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.wrapped); err != nil {
				rows.Close()
				return total, err
			}
			batchRows = append(batchRows, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return total, err
		}
		if len(batchRows) == 0 {
			return total, nil
		}
		err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			for _, r := range batchRows {
				newWrapped, newKeyRef, err := cipher.Rewrap(ctx, r.wrapped)
				if err != nil {
					return fmt.Errorf("row %s: %w", r.id, err)
				}
				if _, err := tx.Exec(ctx, upd, newWrapped, newKeyRef, r.id); err != nil {
					return fmt.Errorf("row %s update: %w", r.id, err)
				}
			}
			return nil
		})
		if err != nil {
			return total, err
		}
		total += len(batchRows)
	}
}

// reindexTable decrypts every un-erased value of one table and rewrites its blind index under the
// cipher's current blind-index key, keyset-paginated by id. Idempotent (recomputing with the same key
// yields the same index), so a re-run after an interruption is safe — it simply recomputes.
func reindexTable(ctx context.Context, pool *pgxpool.Pool, cipher *crypto.Cipher, t encTable, batch int) (int, error) {
	sel := fmt.Sprintf(`SELECT id::text, %s, %s, %s FROM %s WHERE %s IS NOT NULL AND %s IS NOT NULL AND id > $1::uuid ORDER BY id LIMIT $2`,
		t.ciphertext, t.wrappedDEK, t.keyRef, t.name, t.ciphertext, t.wrappedDEK)
	upd := fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE id = $2::uuid`, t.name, t.blindIndex)

	cursor := "00000000-0000-0000-0000-000000000000"
	total := 0
	for {
		type row struct {
			id                  string
			ciphertext, wrapped []byte
			keyRef              string
		}
		var batchRows []row
		rows, err := pool.Query(ctx, sel, cursor, batch)
		if err != nil {
			return total, err
		}
		for rows.Next() {
			var r row
			var keyRef *string
			if err := rows.Scan(&r.id, &r.ciphertext, &r.wrapped, &keyRef); err != nil {
				rows.Close()
				return total, err
			}
			if keyRef != nil {
				r.keyRef = *keyRef
			}
			batchRows = append(batchRows, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return total, err
		}
		if len(batchRows) == 0 {
			return total, nil
		}
		err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			for _, r := range batchRows {
				pt, err := cipher.Open(ctx, crypto.Sealed{Ciphertext: r.ciphertext, WrappedDEK: r.wrapped, KeyRef: r.keyRef})
				if err != nil {
					return fmt.Errorf("row %s open: %w", r.id, err)
				}
				if _, err := tx.Exec(ctx, upd, cipher.BlindIndex(pt), r.id); err != nil {
					return fmt.Errorf("row %s update: %w", r.id, err)
				}
			}
			return nil
		})
		if err != nil {
			return total, err
		}
		cursor = batchRows[len(batchRows)-1].id
		total += len(batchRows)
	}
}
