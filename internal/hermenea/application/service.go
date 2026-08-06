// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application is the hermenea orchestrator (M16 / D-Hermenea): it registers sources/mappers,
// enqueues sync jobs (push trigger + cron), and runs one job's fetch → stage → map → load pipeline
// against oikumenea's import endpoint, recording import_runs lineage. It owns no scheduling loop — the
// runtime package drives it.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

// Service orchestrates ingestion over the store (hermenea's DB), the connector registry, the per
// object-type mapper registries (in-memory + paged), and the loader (oikumenea's import endpoint). It
// also holds the optional live watchlist checker (D-Watchlists, M34), the one synchronous surface.
type Service struct {
	store        domain.Store
	fetchers     map[string]domain.Fetcher
	mappers      map[string]domain.Mapper
	pagedMappers map[string]domain.PagedMapper
	loader       domain.Loader
	watchlist    domain.WatchlistChecker
	reporter     domain.RunReporter // optional connector-plane run reporter (M53); nil → no-op
}

// NewService wires the service with its store, fetcher registry, and loader. Mappers are registered
// per object-type at composition time (RegisterMapper / RegisterPagedMapper).
func NewService(store domain.Store, fetchers map[string]domain.Fetcher, loader domain.Loader) *Service {
	return &Service{
		store:        store,
		fetchers:     fetchers,
		mappers:      map[string]domain.Mapper{},
		pagedMappers: map[string]domain.PagedMapper{},
		loader:       loader,
	}
}

// SetReporter binds the optional connector-plane run reporter (M53 / D-ConnectorPlane). Composition-time;
// a service without one reports nothing (the reporter's methods are no-ops on a nil value).
func (s *Service) SetReporter(r domain.RunReporter) { s.reporter = r }

// SetWatchlistChecker binds the live screening checker (D-Watchlists, M34). Composition-time.
func (s *Service) SetWatchlistChecker(w domain.WatchlistChecker) { s.watchlist = w }

// CheckWatchlist runs a live screening check for a person-identity query (the synchronous
// oikumenea→hermenea surface). Returns ErrNoWatchlist when no checker is configured.
func (s *Service) CheckWatchlist(ctx context.Context, q domain.WatchlistQuery) (domain.WatchlistResult, error) {
	if s.watchlist == nil {
		return domain.WatchlistResult{}, domain.ErrNoWatchlist
	}
	return s.watchlist.Check(ctx, q)
}

// RegisterMapper registers the in-memory raw→records mapper for an object-type (composition-time).
func (s *Service) RegisterMapper(objectType string, m domain.Mapper) { s.mappers[objectType] = m }

// RegisterPagedMapper registers the paged mapper for an object-type (used when the source's connector
// is a StreamingFetcher — the wof-sqlite / geo-places path, D-GeoPlaces).
func (s *Service) RegisterPagedMapper(objectType string, m domain.PagedMapper) {
	s.pagedMappers[objectType] = m
}

// SeedSource upserts a configured source and (when it carries a cron) its schedule. Idempotent.
func (s *Service) SeedSource(ctx context.Context, src domain.Source) error {
	saved, err := s.store.UpsertSource(ctx, src)
	if err != nil {
		return err
	}
	if src.Cron != "" {
		if _, err := domain.ScheduleInterval(src.Cron); err != nil {
			return fmt.Errorf("source %s: %w", src.Code, err)
		}
		return s.store.UpsertSchedule(ctx, saved.ID, src.Cron)
	}
	return nil
}

// ListSources / ListRuns / ListJobs back the read endpoints.
func (s *Service) ListSources(ctx context.Context) ([]domain.Source, error) {
	return s.store.ListSources(ctx)
}
func (s *Service) ListRuns(ctx context.Context, limit int) ([]domain.Run, error) {
	return s.store.ListRuns(ctx, limit)
}
func (s *Service) ListJobs(ctx context.Context, limit int) ([]domain.Job, error) {
	return s.store.ListJobs(ctx, limit)
}

// TriggerSync enqueues a sync job for a source code (the oikumenea push trigger). The idempotency key
// buckets by source + second so a burst of duplicate triggers folds into one queued job.
func (s *Service) TriggerSync(ctx context.Context, code string) (jobID, status string, err error) {
	src, ok, err := s.store.GetSourceByCode(ctx, code)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", domain.ErrUnknownSource
	}
	return s.enqueueSync(ctx, src, fmt.Sprintf("trigger:%s:%d", code, time.Now().Unix()))
}

// EnqueueDueSchedules enqueues a sync for each enabled schedule whose interval has elapsed (called by
// the runtime's scheduler tick). The idempotency key is bucketed to the interval window so one job is
// enqueued per window even across ticks/restarts.
func (s *Service) EnqueueDueSchedules(ctx context.Context, now time.Time) error {
	schedules, err := s.store.ListEnabledSchedules(ctx)
	if err != nil {
		return err
	}
	for _, sch := range schedules {
		interval, err := domain.ScheduleInterval(sch.Cron)
		if err != nil {
			continue // a bad spec is skipped (validated at SeedSource)
		}
		if !domain.ScheduleDue(interval, sch.LastEnqueuedAt, now) {
			continue
		}
		src, ok, err := s.store.GetSourceByCode(ctx, sch.SourceCode)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		key := fmt.Sprintf("cron:%s:%d", sch.SourceCode, now.Truncate(interval).Unix())
		if _, _, err := s.enqueueSync(ctx, src, key); err != nil {
			return err
		}
		if err := s.store.TouchSchedule(ctx, sch.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enqueueSync(ctx context.Context, src domain.Source, idemKey string) (string, string, error) {
	payload, _ := json.Marshal(struct {
		Source string `json:"source"`
	}{Source: src.Code})
	return s.store.EnqueueJob(ctx, domain.JobSync, idemKey, src.ID, payload, defaultMaxAttempts)
}

// defaultMaxAttempts is the dead-letter threshold for sync jobs.
const defaultMaxAttempts = 5

// ProcessJob runs one claimed job to completion: fetch → stage raw → start run → map → load →
// finish run. A returned error tells the worker to retry/backoff (the run is finished as failed).
func (s *Service) ProcessJob(ctx context.Context, job domain.Job) error {
	if job.JobType != domain.JobSync {
		return fmt.Errorf("hermenea: unknown job type %q", job.JobType)
	}
	src, ok, err := s.store.GetSourceByCode(ctx, job.SourceCode)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrUnknownSource
	}
	fetcher, ok := s.fetchers[src.FetcherType]
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrNoFetcher, src.FetcherType)
	}

	// Streaming fetchers (wof-sqlite, D-GeoPlaces) take the paged path: the payload is too large for
	// a single in-memory batch, so the source is staged to disk and loaded one bounded page at a time.
	if sc, ok := fetcher.(domain.StreamingFetcher); ok {
		pm, ok := s.pagedMappers[src.ObjectType]
		if !ok {
			return fmt.Errorf("%w: %s (paged)", domain.ErrNoMapper, src.ObjectType)
		}
		return s.processStreaming(ctx, job, src, sc, pm)
	}

	mapper, ok := s.mappers[src.ObjectType]
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrNoMapper, src.ObjectType)
	}

	raw, err := fetcher.Fetch(ctx, src)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", src.Code, err)
	}
	rawID, err := s.store.InsertRawBatch(ctx, src.ID, raw.SourceVersion, raw.Checksum, raw.Payload)
	if err != nil {
		return err
	}
	runID, err := s.startRun(ctx, src, rawID, raw.SourceVersion)
	if err != nil {
		return err
	}

	records, err := mapper.Map(raw)
	if err != nil {
		_ = s.finishRun(ctx, runID, src, domain.RunFailed, domain.ImportSummary{}, err.Error())
		return fmt.Errorf("map %s: %w", src.Code, err)
	}
	sum, err := s.loader.Load(ctx, src.ObjectType, src.Code, raw.SourceVersion, runID,
		s.resumeSeq(job, raw.Checksum), records, s.cursorAck(job.ID, raw.Checksum))
	if err != nil {
		_ = s.finishRun(ctx, runID, src, domain.RunFailed, domain.ImportSummary{}, err.Error())
		return fmt.Errorf("load %s: %w", src.Code, err)
	}
	return s.finishRun(ctx, runID, src, domain.RunSucceeded, sum, "")
}

// resumeSeq returns the chunk seq to resume a chunked run after (R-05): the job's persisted cursor,
// valid only while the (re-)staged source still carries the checksum the cursor was written against —
// a changed source invalidates it (full, still-idempotent re-run).
func (s *Service) resumeSeq(job domain.Job, checksum string) int {
	if job.ResumeChecksum != "" && job.ResumeChecksum == checksum {
		return job.ResumeSeq
	}
	return 0
}

// cursorAck persists the resume cursor after every chunk oikumenea acknowledged.
func (s *Service) cursorAck(jobID, checksum string) domain.AckFunc {
	return func(ctx context.Context, seq int) error {
		return s.store.SetJobCursor(ctx, jobID, seq, checksum)
	}
}

// processStreaming runs the paged pipeline for a StreamingFetcher source (D-GeoPlaces; chunked
// since R-05): stage the source to disk, record a file-referenced raw batch, then open one chunked
// run — each parent-first page is re-sliced into ≤chunkSize chunks, each chunk its own canonical
// envelope / oikumenea transaction, acked into the job's resume cursor. On a retried attempt the run
// skips the chunks already acked (checksum-guarded), so a mid-run crash of either side resumes
// instead of restarting; the trailing finalize chunk runs the object-type's batch finalizers. A
// chunk-load error fails the run (the worker retries/backs off); the staged file is always removed.
// NOTE: a resumed attempt's import_run row aggregates only the chunks this attempt sent — the failed
// attempt's row holds the earlier counts (the run ledger is per attempt, unchanged).
func (s *Service) processStreaming(ctx context.Context, job domain.Job, src domain.Source, sc domain.StreamingFetcher, pm domain.PagedMapper) error {
	staged, err := sc.Stage(ctx, src)
	if err != nil {
		return fmt.Errorf("stage %s: %w", src.Code, err)
	}
	if staged.Cleanup != nil {
		defer staged.Cleanup()
	}
	rawID, err := s.store.InsertRawBatchRef(ctx, src.ID, staged.SourceVersion, staged.Checksum, staged.Path)
	if err != nil {
		return err
	}
	runID, err := s.startRun(ctx, src, rawID, staged.SourceVersion)
	if err != nil {
		return err
	}

	run := s.loader.StartRun(src.ObjectType, src.Code, staged.SourceVersion, runID,
		s.resumeSeq(job, staged.Checksum), s.cursorAck(job.ID, staged.Checksum))
	loadErr := pm.MapPaged(ctx, staged, func(records []map[string]any) error {
		if len(records) == 0 {
			return nil
		}
		return run.Push(ctx, records)
	})
	var agg domain.ImportSummary
	if loadErr == nil {
		agg, loadErr = run.Finalize(ctx)
	}
	if loadErr != nil {
		_ = s.finishRun(ctx, runID, src, domain.RunFailed, agg, loadErr.Error())
		return fmt.Errorf("stream %s: %w", src.Code, loadErr)
	}
	return s.finishRun(ctx, runID, src, domain.RunSucceeded, agg, "")
}

// startRun opens a run in the store (the connector's own ledger) and reports it as `running` to
// oikumenea's connector registry (M53). The store write is authoritative; the report is best-effort.
func (s *Service) startRun(ctx context.Context, src domain.Source, rawID, sourceVersion string) (string, error) {
	runID, err := s.store.StartRun(ctx, src.ID, rawID, sourceVersion)
	if err != nil {
		return "", err
	}
	s.reportRun(ctx, src.Code, runID, domain.RunRunning, domain.ImportSummary{}, "")
	return runID, nil
}

// finishRun closes a run in the store and reports the terminal state (succeeded/failed) with its counts
// to the connector registry (best-effort). It returns the store error, if any — the report never does.
func (s *Service) finishRun(ctx context.Context, runID string, src domain.Source, state string, sum domain.ImportSummary, errMsg string) error {
	err := s.store.FinishRun(ctx, runID, state, sum.Created, sum.Updated, sum.Skipped, errMsg)
	s.reportRun(ctx, src.Code, runID, state, sum, errMsg)
	return err
}

// reportRun pushes one run state to the connector registry, swallowing any error: reporting is
// best-effort and must NEVER fail the underlying job (D-ConnectorPlane: visibility, not orchestration).
// A nil reporter (unit tests, a core-less install) is skipped outright.
func (s *Service) reportRun(ctx context.Context, sourceCode, runID, state string, sum domain.ImportSummary, errMsg string) {
	if s.reporter == nil {
		return
	}
	if err := s.reporter.ReportRun(ctx, sourceCode, runID, state, sum, errMsg); err != nil {
		svc1log.FromContext(ctx).Warn("hermenea: connector-plane run report failed (visibility only)",
			svc1log.SafeParam("source", sourceCode),
			svc1log.SafeParam("runId", runID),
			svc1log.SafeParam("state", state),
			svc1log.Stacktrace(err))
	}
}
