// Package domain holds the hermenea companion service's framework-free core (M16 / D-Hermenea): the
// ingestion + job-runtime types, the Connector / Mapper / Loader seams, the repository port, the
// backoff policy, and the (dependency-free) schedule-interval parser. Hermenea fetches → stages →
// maps an external dataset and LOADS it into oikumenea over HTTP only (the Loader); it never opens
// oikumenea's database.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Connector types (D-Hermenea; DS-44 parks jdbc-sql/object-store).
const (
	ConnectorHTTP = "http"
	ConnectorFile = "file"
	// ConnectorWOFSQLite stages a Who's-On-First SQLite distribution (a .db.bz2 over HTTP) to local
	// disk for the paged geo-places pipeline (D-GeoPlaces). It is a StreamingConnector: the payload is
	// gigabytes of geometry, so it never lands in memory or the raw-batch BYTEA column.
	ConnectorWOFSQLite = "wof-sqlite"
	// ConnectorHTTPFiles streams a whitespace-separated LIST of URLs to a temp directory for the paged
	// language pipeline (D-Languages, M18): the Glottolog CLDF (languages.csv + values.csv) and the CLDR
	// language→script inputs (supplementalData.xml + iso-639-3.tab) are transformed live in Go on each
	// run. A StreamingConnector — raw values.csv exceeds the 16 MiB in-memory cap, and an object-type
	// needs several files, so the set is staged to disk and read by a PagedMapper.
	ConnectorHTTPFiles = "http-files"
	// ConnectorFactbook enumerates the CIA World Factbook country files (the `factbook/factbook.json`
	// GitHub mirror) via one git-tree API call, then streams each `<region>/<cc>.json` to a temp directory
	// for the paged ethnicity pipeline (D-PhysicalIdentity amendment, M43). A StreamingConnector: ~260
	// country files (whole Factbook docs) exceed the 16 MiB in-memory cap, so the set is staged to disk and
	// parsed by a PagedMapper. The locator is `owner/repo@ref` (default `factbook/factbook.json@master`).
	ConnectorFactbook = "factbook"
)

// Job types in the queue.
const (
	JobSync = "sync"
)

// Worker-job statuses.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobDead      = "dead"
)

// Import-run statuses.
const (
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
)

var (
	// ErrUnknownSource is returned when no source is registered under a code.
	ErrUnknownSource = errors.New("unknown import source")
	// ErrNoMapper is returned when no mapper is registered for a source's object-type.
	ErrNoMapper = errors.New("no mapper registered for object-type")
	// ErrNoConnector is returned when no connector is registered for a source's connector-type.
	ErrNoConnector = errors.New("no connector for connector-type")
	// ErrBadSchedule is returned when a schedule's cron/interval spec cannot be parsed.
	ErrBadSchedule = errors.New("unparsable schedule spec")
)

// Source is a registered external dataset hermenea can sync into oikumenea.
type Source struct {
	ID            string
	Code          string
	Name          string
	ConnectorType string
	ObjectType    string
	Locator       string
	Cron          string // empty = trigger-only
	Enabled       bool
}

// Job is one unit of queued work in the runtime.
type Job struct {
	ID             string
	JobType        string
	IdempotencyKey string
	SourceCode     string
	Status         string
	Attempts       int
	MaxAttempts    int
	RunAfter       time.Time
	LastError      string
}

// Schedule is a cron registration the scheduler enqueues from.
type Schedule struct {
	ID             string
	SourceID       string
	SourceCode     string
	Cron           string
	LastEnqueuedAt *time.Time
}

// Run is one map+load lineage record.
type Run struct {
	ID            string
	SourceCode    string
	SourceVersion string
	Status        string
	Created       int
	Updated       int
	Skipped       int
	Error         string
	StartedAt     time.Time
	FinishedAt    *time.Time
}

// RawBatch is a fetched payload landed verbatim.
type RawBatch struct {
	Payload       []byte
	SourceVersion string
	Checksum      string
}

// ImportSummary is the outcome oikumenea's import endpoint returns to the loader.
type ImportSummary struct {
	Created int
	Updated int
	Skipped int
}

// Connector fetches an external source's raw payload (HTTP download / bundled file).
type Connector interface {
	Fetch(ctx context.Context, src Source) (RawBatch, error)
}

// Mapper turns a raw batch into canonical-envelope records for one object-type.
type Mapper interface {
	Map(raw RawBatch) (records []map[string]any, err error)
}

// StagedSource is a large source landed to local disk rather than in memory (the wof-sqlite path).
// Path is a temp file (wof-sqlite) OR a temp directory of files (http-files, D-Languages); the pipeline
// removes it via Cleanup once the run finishes.
type StagedSource struct {
	Path          string
	SourceVersion string
	Checksum      string
	Cleanup       func()
}

// StreamingConnector stages a large source to disk instead of returning its bytes. A connector that
// implements it is driven by the paged pipeline (its Connector.Fetch is never called). This is how the
// gigabyte-scale WOF gazetteer is ingested without a 16 MiB in-memory batch (D-GeoPlaces).
type StreamingConnector interface {
	Stage(ctx context.Context, src Source) (StagedSource, error)
}

// PageFunc receives one page of canonical records (each page is loaded as its own canonical envelope).
// Returning an error aborts the run (the worker then retries/backs off).
type PageFunc func(records []map[string]any) error

// PagedMapper reads a staged source and emits canonical records in bounded, parent-first pages — the
// total set never lives in memory at once. Registered per object-type (RegisterPagedMapper) and used
// only when the source's connector is a StreamingConnector.
type PagedMapper interface {
	MapPaged(ctx context.Context, staged StagedSource, emit PageFunc) error
}

// Loader pushes a canonical envelope to oikumenea's import endpoint over HTTP (the only oikumenea
// coupling).
type Loader interface {
	Load(ctx context.Context, objectType, source, sourceVersion string, records []map[string]any) (ImportSummary, error)
}

// Store is the hermenea repository port (its OWN database). Adapters implement it.
type Store interface {
	UpsertSource(ctx context.Context, s Source) (Source, error)
	ListSources(ctx context.Context) ([]Source, error)
	GetSourceByCode(ctx context.Context, code string) (Source, bool, error)
	UpsertSchedule(ctx context.Context, sourceID, cron string) error
	ListEnabledSchedules(ctx context.Context) ([]Schedule, error)
	TouchSchedule(ctx context.Context, id string) error
	EnqueueJob(ctx context.Context, jobType, idempotencyKey, sourceID string, payload []byte, maxAttempts int) (jobID, status string, err error)
	ClaimJob(ctx context.Context, worker string) (Job, bool, error)
	MarkJobSucceeded(ctx context.Context, id string) error
	RescheduleJob(ctx context.Context, id string, runAfter time.Time, lastErr string) error
	DeadLetterJob(ctx context.Context, id, lastErr string) error
	ListJobs(ctx context.Context, limit int) ([]Job, error)
	UnhealthyJobs(ctx context.Context) (int, error)
	RequeueStaleRunning(ctx context.Context, lockedBefore time.Time) error
	InsertRawBatch(ctx context.Context, sourceID, sourceVersion, checksum string, payload []byte) (string, error)
	// InsertRawBatchRef stages a large fetched source by file reference (staged_path) instead of inline
	// bytes — the streaming/paged path (D-GeoPlaces). The file itself is transient (removed after the run).
	InsertRawBatchRef(ctx context.Context, sourceID, sourceVersion, checksum, stagedPath string) (string, error)
	StartRun(ctx context.Context, sourceID, rawBatchID, sourceVersion string) (string, error)
	FinishRun(ctx context.Context, id, status string, created, updated, skipped int, errMsg string) error
	ListRuns(ctx context.Context, limit int) ([]Run, error)
}

// Backoff returns the delay before the next attempt: exponential base*2^(attempts-1), capped at max.
// per-job-type tuning is applied by the worker passing that type's base/max (D-Hermenea).
func Backoff(attempts int, base, max time.Duration) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	d := base
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	if d > max {
		return max
	}
	return d
}

// ScheduleInterval parses a hermenea schedule spec into a fixed interval. Supported (dependency-free):
// `@every <dur>` (Go duration, e.g. @every 30m), and the aliases `@hourly`/`@daily`/`@weekly`. Full
// crontab syntax is a documented future seam. Returns ErrBadSchedule on anything else.
func ScheduleInterval(spec string) (time.Duration, error) {
	s := strings.TrimSpace(spec)
	switch s {
	case "@hourly":
		return time.Hour, nil
	case "@daily", "@midnight":
		return 24 * time.Hour, nil
	case "@weekly":
		return 7 * 24 * time.Hour, nil
	}
	if rest, ok := strings.CutPrefix(s, "@every "); ok {
		d, err := time.ParseDuration(strings.TrimSpace(rest))
		if err != nil || d <= 0 {
			return 0, ErrBadSchedule
		}
		return d, nil
	}
	return 0, ErrBadSchedule
}

// ScheduleDue reports whether a schedule with the given interval is due to enqueue now, given when it
// last enqueued (nil = never → due immediately).
func ScheduleDue(interval time.Duration, lastEnqueued *time.Time, now time.Time) bool {
	if lastEnqueued == nil {
		return true
	}
	return now.Sub(*lastEnqueued) >= interval
}
