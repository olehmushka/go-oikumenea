// Package runtime is hermenea's background-job engine (M16 / D-Hermenea; N-worker since R-13): a
// configurable number of worker goroutines claim jobs from the queue (FOR UPDATE SKIP LOCKED,
// at-least-once — the claim is already safe under any number of claimers, in-process or across
// replicas) and one cron scheduler enqueues due syncs (idempotent per interval bucket, so extra
// replicas' schedulers fold into the same jobs). It implements retry with exponential backoff
// (per-job-type config), dead-lettering after max attempts, and GRACEFUL DRAIN — on shutdown it
// stops claiming and lets the in-flight jobs finish (bounded by jobTimeout) before returning.
package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/application"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

// Config tunes the runtime loops and the retry policy.
type Config struct {
	WorkerID     string        // identifies this process in worker_jobs.locked_by (suffixed -w<i> per worker goroutine)
	Concurrency  int           // parallel worker goroutines over the SKIP LOCKED queue (R-13; <=0 → 1)
	PollInterval time.Duration // queue poll cadence when idle
	ScheduleTick time.Duration // cron evaluation cadence
	BackoffBase  time.Duration // first-retry delay (per-job-type base; doubled each attempt)
	BackoffMax   time.Duration // backoff cap
	JobTimeout   time.Duration // per-job hard timeout (bounds graceful drain)
	StaleAfter   time.Duration // requeue jobs left 'running' longer than this (crash recovery)
}

// Runtime owns the worker + scheduler goroutines.
type Runtime struct {
	svc   *application.Service
	store domain.Store
	cfg   Config
	wg    sync.WaitGroup
}

// New builds the runtime.
func New(svc *application.Service, store domain.Store, cfg Config) *Runtime {
	return &Runtime{svc: svc, store: store, cfg: cfg}
}

// Start launches Concurrency worker goroutines + the scheduler goroutine bound to ctx. Returns a Stop
// that cancels them and waits for the in-flight jobs to drain. ctx is the long-lived server context;
// Stop is wired as the witchcraft cleanup so shutdown drains cleanly.
func (r *Runtime) Start(ctx context.Context) (stop func()) {
	loopCtx, cancel := context.WithCancel(ctx)
	n := r.cfg.Concurrency
	if n <= 0 {
		n = 1
	}
	r.wg.Add(n + 1)
	for i := 0; i < n; i++ {
		go r.worker(loopCtx, fmt.Sprintf("%s-w%d", r.cfg.WorkerID, i+1))
	}
	go r.scheduler(loopCtx)
	return func() {
		cancel()
		r.wg.Wait()
	}
}

// worker claims and runs one job at a time (N workers = N jobs in parallel; the SKIP LOCKED claim
// guarantees no job is handed to two workers). On ctx cancel it stops claiming (graceful drain): a job
// already claimed runs to completion under its own jobTimeout, not the cancelled loop context.
func (r *Runtime) worker(ctx context.Context, workerID string) {
	defer r.wg.Done()
	logger := svc1log.FromContext(ctx)
	ticker := time.NewTicker(r.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		job, ok, err := r.store.ClaimJob(ctx, workerID)
		if err != nil {
			logger.Warn("hermenea worker: claim failed", svc1log.Stacktrace(err))
		} else if ok {
			r.runJob(job, logger)
			continue // drain the queue without waiting when work is available
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// runJob executes one job under a fresh timeout context (so it completes even during shutdown drain),
// then records success / retry-with-backoff / dead-letter.
func (r *Runtime) runJob(job domain.Job, logger svc1log.Logger) {
	jobCtx, cancel := context.WithTimeout(context.Background(), r.cfg.JobTimeout)
	defer cancel()
	jobCtx = svc1log.WithLogger(jobCtx, logger)

	err := r.svc.ProcessJob(jobCtx, job)
	if err == nil {
		if e := r.store.MarkJobSucceeded(context.Background(), job.ID); e != nil {
			logger.Error("hermenea worker: mark succeeded failed", svc1log.Stacktrace(e))
		}
		return
	}
	if job.Attempts >= job.MaxAttempts {
		logger.Error("hermenea worker: job dead-lettered", svc1log.SafeParam("jobId", job.ID), svc1log.Stacktrace(err))
		_ = r.store.DeadLetterJob(context.Background(), job.ID, err.Error())
		return
	}
	delay := domain.Backoff(job.Attempts, r.cfg.BackoffBase, r.cfg.BackoffMax)
	logger.Warn("hermenea worker: job failed, retrying",
		svc1log.SafeParam("jobId", job.ID), svc1log.SafeParam("attempt", job.Attempts), svc1log.SafeParam("retryInMs", delay.Milliseconds()), svc1log.Stacktrace(err))
	_ = r.store.RescheduleJob(context.Background(), job.ID, time.Now().Add(delay), err.Error())
}

// scheduler evaluates cron schedules each tick and enqueues due syncs. It also requeues stale
// 'running' jobs (a previous process crashed mid-job) so they retry.
func (r *Runtime) scheduler(ctx context.Context) {
	defer r.wg.Done()
	logger := svc1log.FromContext(ctx)
	ticker := time.NewTicker(r.cfg.ScheduleTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if r.cfg.StaleAfter > 0 {
				_ = r.store.RequeueStaleRunning(ctx, now.Add(-r.cfg.StaleAfter))
			}
			if err := r.svc.EnqueueDueSchedules(ctx, now); err != nil {
				logger.Warn("hermenea scheduler: enqueue due failed", svc1log.Stacktrace(err))
			}
		}
	}
}
