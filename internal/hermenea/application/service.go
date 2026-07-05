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

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// Service orchestrates ingestion over the store (hermenea's DB), the connector registry, the per
// object-type mapper registries (in-memory + paged), and the loader (oikumenea's import endpoint). It
// also holds the optional live watchlist checker (D-Watchlists, M34), the one synchronous surface.
type Service struct {
	store        domain.Store
	connectors   map[string]domain.Connector
	mappers      map[string]domain.Mapper
	pagedMappers map[string]domain.PagedMapper
	loader       domain.Loader
	watchlist    domain.WatchlistChecker
}

// NewService wires the service with its store, connector registry, and loader. Mappers are registered
// per object-type at composition time (RegisterMapper / RegisterPagedMapper).
func NewService(store domain.Store, connectors map[string]domain.Connector, loader domain.Loader) *Service {
	return &Service{
		store:        store,
		connectors:   connectors,
		mappers:      map[string]domain.Mapper{},
		pagedMappers: map[string]domain.PagedMapper{},
		loader:       loader,
	}
}

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
// is a StreamingConnector — the wof-sqlite / geo-places path, D-GeoPlaces).
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
	conn, ok := s.connectors[src.ConnectorType]
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrNoConnector, src.ConnectorType)
	}

	// Streaming connectors (wof-sqlite, D-GeoPlaces) take the paged path: the payload is too large for
	// a single in-memory batch, so the source is staged to disk and loaded one bounded page at a time.
	if sc, ok := conn.(domain.StreamingConnector); ok {
		pm, ok := s.pagedMappers[src.ObjectType]
		if !ok {
			return fmt.Errorf("%w: %s (paged)", domain.ErrNoMapper, src.ObjectType)
		}
		return s.processStreaming(ctx, src, sc, pm)
	}

	mapper, ok := s.mappers[src.ObjectType]
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrNoMapper, src.ObjectType)
	}

	raw, err := conn.Fetch(ctx, src)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", src.Code, err)
	}
	rawID, err := s.store.InsertRawBatch(ctx, src.ID, raw.SourceVersion, raw.Checksum, raw.Payload)
	if err != nil {
		return err
	}
	runID, err := s.store.StartRun(ctx, src.ID, rawID, raw.SourceVersion)
	if err != nil {
		return err
	}

	records, err := mapper.Map(raw)
	if err != nil {
		_ = s.store.FinishRun(ctx, runID, domain.RunFailed, 0, 0, 0, err.Error())
		return fmt.Errorf("map %s: %w", src.Code, err)
	}
	sum, err := s.loader.Load(ctx, src.ObjectType, src.Code, raw.SourceVersion, records)
	if err != nil {
		_ = s.store.FinishRun(ctx, runID, domain.RunFailed, 0, 0, 0, err.Error())
		return fmt.Errorf("load %s: %w", src.Code, err)
	}
	return s.store.FinishRun(ctx, runID, domain.RunSucceeded, sum.Created, sum.Updated, sum.Skipped, "")
}

// processStreaming runs the paged pipeline for a StreamingConnector source (D-GeoPlaces): stage the
// source to disk, record a file-referenced raw batch, then load each parent-first page as its own
// canonical envelope, aggregating the upsert counts into one import_run. A page-load error fails the
// run (the worker retries/backs off); the staged file is always removed.
func (s *Service) processStreaming(ctx context.Context, src domain.Source, sc domain.StreamingConnector, pm domain.PagedMapper) error {
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
	runID, err := s.store.StartRun(ctx, src.ID, rawID, staged.SourceVersion)
	if err != nil {
		return err
	}

	var agg domain.ImportSummary
	loadErr := pm.MapPaged(ctx, staged, func(records []map[string]any) error {
		if len(records) == 0 {
			return nil
		}
		sum, err := s.loader.Load(ctx, src.ObjectType, src.Code, staged.SourceVersion, records)
		if err != nil {
			return err
		}
		agg.Created += sum.Created
		agg.Updated += sum.Updated
		agg.Skipped += sum.Skipped
		return nil
	})
	if loadErr != nil {
		_ = s.store.FinishRun(ctx, runID, domain.RunFailed, agg.Created, agg.Updated, agg.Skipped, loadErr.Error())
		return fmt.Errorf("stream %s: %w", src.Code, loadErr)
	}
	return s.store.FinishRun(ctx, runID, domain.RunSucceeded, agg.Created, agg.Updated, agg.Skipped, "")
}
