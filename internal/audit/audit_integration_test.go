//go:build integration

// Integration tests for the audit module against a real Postgres (M1 exit criteria, D-Audit):
//   - a write + its audit row share one transaction — they commit or roll back together;
//   - the read path filters and token-paginates.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5544/postgres?sslmode=disable" \
//	  go test -tags integration ./internal/audit/...
package audit_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	"github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5544/postgres?sslmode=disable"

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

	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, func() int { return 50 })
	return svc, pool
}

// mintActionRID composes a fresh action RID via the same SQL generator the producing modules use.
func mintActionRID(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var rid string
	if err := pool.QueryRow(ctx, "SELECT oikumenea.new_rid('audit', 'action__test')").Scan(&rid); err != nil {
		t.Fatalf("mint action rid: %v", err)
	}
	return rid
}

func personEntry(id, personID string) domain.Entry {
	return domain.Entry{
		ID:            id,
		ActorType:     domain.ActorPerson,
		ActorPersonID: personID,
		Action:        "assignment.grant",
		TargetType:    "role_assignment",
		RequestID:     "req-" + uuid.NewString(),
		Outcome:       domain.OutcomeSuccess,
	}
}

// TestRecordSharesTransactionFate is the headline D-Audit guarantee: the audit row lives or dies
// with the caller's transaction.
func TestRecordSharesTransactionFate(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	personID := "urn:oikumenea:person:local:person:" + uuid.NewString()

	// Rolled-back transaction: the entry must NOT be readable afterward.
	rolledBackID := mintActionRID(t, ctx, pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := svc.Record(ctx, tx, personEntry(rolledBackID, personID)); err != nil {
		t.Fatalf("record (to be rolled back): %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := svc.Get(ctx, rolledBackID); err != domain.ErrNotFound {
		t.Fatalf("rolled-back entry should be absent, got err=%v", err)
	}

	// Committed transaction: the entry must be readable, with payload intact.
	committedID := mintActionRID(t, ctx, pool)
	e := personEntry(committedID, personID)
	e.After = json.RawMessage(`{"role":"unit-admin"}`)
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin2: %v", err)
	}
	if err := svc.Record(ctx, tx2, e); err != nil {
		t.Fatalf("record (to be committed): %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	got, err := svc.Get(ctx, committedID)
	if err != nil {
		t.Fatalf("get committed: %v", err)
	}
	if got.ID != committedID || got.ActorType != domain.ActorPerson || got.ActorPersonID != personID {
		t.Fatalf("unexpected entry: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatalf("created_at not populated")
	}
	if string(got.After) == "" {
		t.Fatalf("after payload lost")
	}
}

// TestQueryPaginates seeds a known set and walks it newest-first in bounded pages.
func TestQueryPaginates(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	personID := "urn:oikumenea:person:local:person:" + uuid.NewString()

	const total = 5
	want := make(map[string]bool, total)
	for i := 0; i < total; i++ {
		id := mintActionRID(t, ctx, pool)
		if err := svc.Record(ctx, pool, personEntry(id, personID)); err != nil {
			t.Fatalf("seed record %d: %v", i, err)
		}
		want[id] = true
		time.Sleep(time.Millisecond) // distinct created_at for a stable, human-meaningful order
	}

	filter := &personID
	seen := make(map[string]bool, total)
	var lastCreated time.Time
	token := ""
	pages := 0
	for {
		page, err := svc.Query(ctx, application.QueryParams{ActorPersonID: filter, PageSize: 2, PageToken: token})
		if err != nil {
			t.Fatalf("query page %d: %v", pages, err)
		}
		pages++
		for _, e := range page.Entries {
			if seen[e.ID] {
				t.Fatalf("duplicate entry across pages: %s", e.ID)
			}
			seen[e.ID] = true
			if !lastCreated.IsZero() && e.CreatedAt.After(lastCreated) {
				t.Fatalf("entries not newest-first: %s", e.ID)
			}
			lastCreated = e.CreatedAt
		}
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
		if pages > total+2 {
			t.Fatal("pagination did not terminate")
		}
	}

	if len(seen) != total {
		t.Fatalf("expected %d entries, saw %d", total, len(seen))
	}
	for id := range want {
		if !seen[id] {
			t.Fatalf("missing seeded entry %s", id)
		}
	}
	if pages < 3 { // 2 + 2 + 1 with pageSize 2 over 5 rows
		t.Fatalf("expected at least 3 pages, got %d", pages)
	}
}

// TestRecordRejectsInvalidEntry exercises the domain invariant (a person actor with a subsystem).
func TestRecordRejectsInvalidEntry(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	bad := personEntry(mintActionRID(t, ctx, pool), "urn:oikumenea:person:local:person:"+uuid.NewString())
	bad.Subsystem = "bootstrap" // person + subsystem is the forbidden shape
	if err := svc.Record(ctx, pool, bad); err == nil {
		t.Fatal("expected validation error for person actor with subsystem")
	}
}
