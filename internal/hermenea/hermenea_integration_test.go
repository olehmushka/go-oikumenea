// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

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
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/adapters"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/application"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/fetcher"
	hdb "github.com/olegamysk/go-oikumenea/internal/hermenea/db"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/geocountries"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/runtime"
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

func (l *stubLoader) Load(_ context.Context, objectType, _, _, _ string, _ int, records []map[string]any, _ domain.AckFunc) (domain.ImportSummary, error) {
	l.gotType = objectType
	l.gotRecords = len(records)
	if l.err != nil {
		return domain.ImportSummary{}, l.err
	}
	return l.summary, nil
}

func (l *stubLoader) StartRun(objectType, _, _, _ string, _ int, _ domain.AckFunc) domain.LoadRun {
	return &stubRun{l: l, objectType: objectType}
}

// stubRun mirrors stubLoader for the chunked-run (streaming) path.
type stubRun struct {
	l          *stubLoader
	objectType string
}

func (r *stubRun) Push(_ context.Context, records []map[string]any) error {
	r.l.gotType = r.objectType
	r.l.gotRecords += len(records)
	return r.l.err
}

func (r *stubRun) Finalize(context.Context) (domain.ImportSummary, error) {
	if r.l.err != nil {
		return domain.ImportSummary{}, r.l.err
	}
	return r.l.summary, nil
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
	svc := application.NewService(store, fetcher.Default(), loader)
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
		FetcherType: domain.FetcherHTTP,
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
		FetcherType: domain.FetcherFile,
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
		FetcherType: domain.FetcherFile,
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

// TestJobResumeCursorRoundTrip proves the chunked-run resume cursor (R-05) survives the
// fail→reschedule→re-claim cycle: SetJobCursor persists (seq, checksum) on the job row and the next
// ClaimJob hands them back, so a retried attempt can skip already-acked chunks.
func TestJobResumeCursorRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)

	key := "cursor-test-" + uuid.NewString()
	jobID, _, err := store.EnqueueJob(ctx, domain.JobSync, key, "", []byte(`{"source":"cursor-src"}`), 5)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Claim until we get OUR job (parking any unrelated leftovers far in the future).
	claim := func() domain.Job {
		t.Helper()
		for i := 0; i < 25; i++ {
			job, ok, err := store.ClaimJob(ctx, "cursor-worker")
			if err != nil {
				t.Fatalf("claim: %v", err)
			}
			if !ok {
				t.Fatalf("queue empty before finding job %s", jobID)
			}
			if job.ID == jobID {
				return job
			}
			_ = store.RescheduleJob(ctx, job.ID, time.Now().Add(time.Hour), "parked by cursor test")
		}
		t.Fatalf("did not claim job %s in 25 attempts", jobID)
		return domain.Job{}
	}

	job := claim()
	if job.ResumeSeq != 0 || job.ResumeChecksum != "" {
		t.Fatalf("fresh job carries a cursor: %+v", job)
	}
	if err := store.SetJobCursor(ctx, jobID, 7, "sum-abc"); err != nil {
		t.Fatalf("set cursor: %v", err)
	}
	// The attempt "fails" → reschedule now → the retry must see the cursor.
	if err := store.RescheduleJob(ctx, jobID, time.Now().Add(-time.Second), "simulated chunk failure"); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	job = claim()
	if job.ResumeSeq != 7 || job.ResumeChecksum != "sum-abc" {
		t.Fatalf("retried job cursor = (%d, %q), want (7, sum-abc)", job.ResumeSeq, job.ResumeChecksum)
	}
	_ = store.MarkJobSucceeded(ctx, jobID)
}

// barrierMapper blocks every Map call until `need` calls have arrived (or times out) — the test
// only passes when that many jobs are being processed AT THE SAME TIME.
type barrierMapper struct {
	need    int32
	arrived atomic.Int32
}

func (m *barrierMapper) Map(domain.RawBatch) ([]map[string]any, error) {
	m.arrived.Add(1)
	deadline := time.Now().Add(10 * time.Second)
	for m.arrived.Load() < m.need {
		if time.Now().After(deadline) {
			return nil, errors.New("barrier timeout: not enough concurrent workers")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return []map[string]any{{"ok": true}}, nil
}

// TestWorkerConcurrency proves the R-13 fan-out: with Concurrency=4 the runtime processes 4 jobs in
// parallel (each job's mapper blocks until all 4 are in flight — a single-worker runtime would
// deadlock the barrier and fail). Claim safety across the 4 workers is the SKIP LOCKED guarantee.
func TestWorkerConcurrency(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)
	loader := &stubLoader{summary: domain.ImportSummary{Created: 1}}
	svc := newService(store, loader)
	const objectType = "conc-test-type"
	barrier := &barrierMapper{need: 4}
	svc.RegisterMapper(objectType, barrier)

	// Park any queued leftovers so all 4 workers are free for OUR jobs.
	for {
		job, ok, err := store.ClaimJob(ctx, "conc-drain")
		if err != nil {
			t.Fatalf("drain claim: %v", err)
		}
		if !ok {
			break
		}
		_ = store.RescheduleJob(ctx, job.ID, time.Now().Add(time.Hour), "parked by concurrency test")
	}

	codes := make([]string, 4)
	for i := range codes {
		src := domain.Source{
			Code:          fmt.Sprintf("conc-%d-%s", i, uuid.NewString()[:8]),
			Name:          "concurrency probe",
			FetcherType: domain.FetcherFile,
			ObjectType:    objectType,
			Locator:       presetFile(t),
			Enabled:       true,
		}
		if err := svc.SeedSource(ctx, src); err != nil {
			t.Fatalf("seed source %d: %v", i, err)
		}
		if _, _, err := svc.TriggerSync(ctx, src.Code); err != nil {
			t.Fatalf("trigger %d: %v", i, err)
		}
		codes[i] = src.Code
	}

	rt := runtime.New(svc, store, runtime.Config{
		WorkerID:     "conc-test",
		Concurrency:  4,
		PollInterval: 20 * time.Millisecond,
		ScheduleTick: time.Hour, // keep the scheduler quiet
		BackoffBase:  50 * time.Millisecond,
		BackoffMax:   time.Second,
		JobTimeout:   15 * time.Second,
	})
	stop := rt.Start(ctx)
	defer stop()

	deadline := time.Now().Add(20 * time.Second)
	for {
		if barrier.arrived.Load() >= 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d jobs in flight after 20s, want 4 concurrent", barrier.arrived.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}
	// All 4 entered the barrier simultaneously — the fan-out works. Wait for clean completion.
	for _, code := range codes {
		waitRunSucceeded(t, store, code, 10*time.Second)
	}
}

// waitRunSucceeded polls the run ledger until the source's latest run succeeds. (Run.SourceCode
// carries the source ID — mirror latestRun's resolution.)
func waitRunSucceeded(t *testing.T, store *adapters.Repository, sourceCode string, timeout time.Duration) {
	t.Helper()
	id := sourceID(t, store, sourceCode)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runs, err := store.ListRuns(context.Background(), 200)
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		for _, r := range runs {
			if r.SourceCode == id && r.Status == domain.RunSucceeded {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("source %s: no succeeded run within %s", sourceCode, timeout)
}
