//go:build integration

// Integration tests for the hermenea companion against its OWN Postgres (M16 exit criteria,
// D-Hermenea). They exercise the job runtime + ingestion orchestration end-to-end WITHOUT oikumenea:
// the oikumenea import endpoint is replaced by an in-test stub Loader, so the focus is hermenea's own
// queue, idempotency, the fetch->map->load->lineage pipeline, and failure handling.
//
//   - a push trigger enqueues a worker_job; a duplicate trigger in the same second folds to one job
//     (idempotency key);
//   - claiming + processing a `file`-connector geo-countries source runs fetch -> map -> load(stub) and
//     writes a succeeded import_runs row carrying the stub's upsert counts;
//   - a loader failure finishes the run `failed` and surfaces the error (the worker then backs off).
//
// Run against a throwaway hermenea DB that has migrations/hermenea applied:
//
//	HERMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/hermenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/hermenea/...
package hermenea_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/adapters"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/application"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/connector"
	hdb "github.com/olegamysk/go-oikumenea/internal/hermenea/db"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/geocountries"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/wikidataorgs"
)

const defaultHermeneaTestDSN = "postgres://postgres:dev@localhost:5432/hermenea_test?sslmode=disable"

// stubLoader stands in for oikumenea's import endpoint: it records the last call and returns a
// configurable summary or error.
type stubLoader struct {
	summary    domain.ImportSummary
	err        error
	gotType    string
	gotRecords int
}

func (l *stubLoader) Load(_ context.Context, objectType, _, _ string, records []map[string]any) (domain.ImportSummary, error) {
	l.gotType = objectType
	l.gotRecords = len(records)
	if l.err != nil {
		return domain.ImportSummary{}, l.err
	}
	return l.summary, nil
}

func newStore(t *testing.T) (*adapters.Repository, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("HERMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultHermeneaTestDSN
	}
	pool, err := hdb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect hermenea test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return adapters.NewRepository(pool), pool
}

// presetFile writes a tiny ISO-3166 source the `file` connector reads (a unique temp path per test).
func presetFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "iso-3166.json")
	if err := os.WriteFile(path, []byte(`[{"alpha2":"ZZ","name":"Testland"},{"alpha2":"YY","name":"Otherland"}]`), 0o600); err != nil {
		t.Fatalf("write preset: %v", err)
	}
	return path
}

func newService(store domain.Store, loader domain.Loader) *application.Service {
	svc := application.NewService(store, connector.Default(), loader)
	svc.RegisterMapper(geocountries.ObjectType, geocountries.Mapper{})
	svc.RegisterMapper(wikidataorgs.ObjectType, wikidataorgs.Mapper{})
	return svc
}

// TestExternalOrgsHTTPConnectorPipeline proves the M30 Wikidata connector path end-to-end through the
// REAL http connector (D-ExternalOrgs): a local server stands in for the Wikidata SPARQL endpoint and
// returns a real-shaped result set; the source's `http` connector fetches it (with the descriptive UA),
// the wikidataorgs mapper transforms it, and the stub loader receives canonical external-organizations
// records keyed by Wikidata Q-id. The DB-side idempotent upsert is proven separately in the externalorg
// integration test.
func TestExternalOrgsHTTPConnectorPipeline(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)

	// A local stand-in for query.wikidata.org returning a real-shaped SPARQL JSON result set. It also
	// asserts the connector sent the descriptive User-Agent (Wikidata 403s a default UA).
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/sparql-results+json")
		_, _ = w.Write([]byte(`{
		  "head": {"vars": ["org","orgLabel","countryCode","kind"]},
		  "results": {"bindings": [
		    {"org":{"type":"uri","value":"http://www.wikidata.org/entity/Q4266"},
		     "orgLabel":{"xml:lang":"en","type":"literal","value":"Party of Regions"},
		     "countryCode":{"type":"literal","value":"UA"},
		     "kind":{"type":"literal","value":"party"}},
		    {"org":{"type":"uri","value":"http://www.wikidata.org/entity/Q1808487"},
		     "orgLabel":{"xml:lang":"en","type":"literal","value":"Ministry of Defence of Ukraine"},
		     "kind":{"type":"literal","value":"government_body"}}
		  ]}
		}`))
	}))
	defer srv.Close()

	loader := &stubLoader{summary: domain.ImportSummary{Created: 2}}
	svc := newService(store, loader)

	src := domain.Source{
		Code:          "wikidata-orgs-" + uuid.NewString()[:8],
		Name:          "test wikidata orgs",
		ConnectorType: domain.ConnectorHTTP,
		ObjectType:    wikidataorgs.ObjectType,
		Locator:       srv.URL + "/sparql?format=json&query=SELECT",
		Enabled:       true,
	}
	if err := svc.SeedSource(ctx, src); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if _, _, err := svc.TriggerSync(ctx, src.Code); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	job, ok, err := store.ClaimJob(ctx, "test-worker")
	if err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	if err := svc.ProcessJob(ctx, job); err != nil {
		t.Fatalf("process job: %v", err)
	}

	if gotUA == "" || gotUA == "Go-http-client/1.1" {
		t.Fatalf("connector did not send a descriptive User-Agent, got %q", gotUA)
	}
	if loader.gotType != wikidataorgs.ObjectType || loader.gotRecords != 2 {
		t.Fatalf("loader got type=%s records=%d, want external-organizations/2", loader.gotType, loader.gotRecords)
	}
	run := latestRun(t, store, src.Code)
	if run.Status != domain.RunSucceeded || run.Created != 2 {
		t.Fatalf("run = %+v, want succeeded Created=2", run)
	}
}

func TestTriggerDedupAndProcess(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)
	loader := &stubLoader{summary: domain.ImportSummary{Created: 2}}
	svc := newService(store, loader)

	src := domain.Source{
		Code:          "geo-countries-" + uuid.NewString()[:8],
		Name:          "test iso-3166",
		ConnectorType: domain.ConnectorFile,
		ObjectType:    geocountries.ObjectType,
		Locator:       presetFile(t),
		Enabled:       true,
	}
	if err := svc.SeedSource(ctx, src); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	// Two triggers in the same second fold to one queued job (idempotency key bucketed by second).
	job1, _, err := svc.TriggerSync(ctx, src.Code)
	if err != nil {
		t.Fatalf("trigger 1: %v", err)
	}
	job2, _, err := svc.TriggerSync(ctx, src.Code)
	if err != nil {
		t.Fatalf("trigger 2: %v", err)
	}
	if job1 != job2 {
		t.Fatalf("duplicate trigger enqueued distinct jobs: %s != %s", job1, job2)
	}

	// Claim + process: fetch(file) -> map(geo-countries) -> load(stub) -> succeeded run.
	job, ok, err := store.ClaimJob(ctx, "test-worker")
	if err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	if err := svc.ProcessJob(ctx, job); err != nil {
		t.Fatalf("process job: %v", err)
	}
	if loader.gotType != geocountries.ObjectType || loader.gotRecords != 2 {
		t.Fatalf("loader got type=%s records=%d, want geo-countries/2", loader.gotType, loader.gotRecords)
	}

	// A succeeded import_runs row records the stub's counts.
	run := latestRun(t, store, src.Code)
	if run.Status != domain.RunSucceeded || run.Created != 2 {
		t.Fatalf("run = %+v, want succeeded Created=2", run)
	}
}

// sourceID resolves a seeded source's RID (import_runs reference the source by id).
func sourceID(t *testing.T, store *adapters.Repository, code string) string {
	t.Helper()
	src, ok, err := store.GetSourceByCode(context.Background(), code)
	if err != nil || !ok {
		t.Fatalf("resolve source %s: ok=%v err=%v", code, ok, err)
	}
	return src.ID
}

func TestProcessLoaderFailureMarksRunFailed(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)
	loadErr := errors.New("oikumenea import 503")
	loader := &stubLoader{err: loadErr}
	svc := newService(store, loader)

	src := domain.Source{
		Code:          "geo-countries-fail-" + uuid.NewString()[:8],
		Name:          "test iso-3166 failing",
		ConnectorType: domain.ConnectorFile,
		ObjectType:    geocountries.ObjectType,
		Locator:       presetFile(t),
		Enabled:       true,
	}
	if err := svc.SeedSource(ctx, src); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if _, _, err := svc.TriggerSync(ctx, src.Code); err != nil {
		t.Fatalf("trigger: %v", err)
	}

	job, ok, err := store.ClaimJob(ctx, "test-worker")
	if err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	if err := svc.ProcessJob(ctx, job); err == nil {
		t.Fatal("expected ProcessJob to surface the loader error")
	}
	run := latestRun(t, store, src.Code)
	if run.Status != domain.RunFailed || run.Error == "" {
		t.Fatalf("run = %+v, want failed with error", run)
	}
}

// latestRun returns the most recent import_runs row for a source code (ListRuns is newest-first). Runs
// reference the source by RID (Run.SourceCode currently carries the source id — see the repository),
// so we resolve the code to its id first.
func latestRun(t *testing.T, store *adapters.Repository, sourceCode string) domain.Run {
	t.Helper()
	id := sourceID(t, store, sourceCode)
	runs, err := store.ListRuns(context.Background(), 50)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	for _, r := range runs {
		if r.SourceCode == id {
			return r
		}
	}
	t.Fatalf("no import_runs row for source %s (id %s)", sourceCode, id)
	return domain.Run{}
}
