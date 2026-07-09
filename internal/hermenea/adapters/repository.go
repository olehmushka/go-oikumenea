// Package adapters implements the hermenea domain ports against its OWN PostgreSQL (M16 / D-Hermenea).
// Generated sqlc code lives in the hermeneasql subpackage and is never hand-edited. The worker-job
// payload carries the source code (JSON) so the worker resolves the source without a join.
package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/adapters/hermeneasql"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Store, bound to a single db.DBTX.
type Repository struct {
	q *hermeneasql.Queries
}

// NewRepository binds a repository to the given command surface (the hermenea pool).
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: hermeneasql.New(conn)}
}

var _ domain.Store = (*Repository)(nil)

// jobPayload is the JSON carried on a sync job so the worker resolves its source without a join.
type jobPayload struct {
	Source string `json:"source"`
}

func (r *Repository) UpsertSource(ctx context.Context, s domain.Source) (domain.Source, error) {
	row, err := r.q.UpsertSource(ctx, hermeneasql.UpsertSourceParams{
		Code:          s.Code,
		Name:          s.Name,
		ConnectorType: s.ConnectorType,
		ObjectType:    s.ObjectType,
		Locator:       s.Locator,
		Cron:          text(s.Cron),
		Enabled:       s.Enabled,
	})
	if err != nil {
		return domain.Source{}, err
	}
	return sourceFrom(row), nil
}

func (r *Repository) ListSources(ctx context.Context) ([]domain.Source, error) {
	rows, err := r.q.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Source, 0, len(rows))
	for _, row := range rows {
		out = append(out, sourceFrom(row))
	}
	return out, nil
}

func (r *Repository) GetSourceByCode(ctx context.Context, code string) (domain.Source, bool, error) {
	row, err := r.q.GetSourceByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Source{}, false, nil
	}
	if err != nil {
		return domain.Source{}, false, err
	}
	return sourceFrom(row), true, nil
}

func (r *Repository) UpsertSchedule(ctx context.Context, sourceID, cron string) error {
	return r.q.UpsertSchedule(ctx, hermeneasql.UpsertScheduleParams{SourceID: sourceID, Cron: cron})
}

func (r *Repository) ListEnabledSchedules(ctx context.Context) ([]domain.Schedule, error) {
	rows, err := r.q.ListEnabledSchedules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Schedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Schedule{
			ID:             row.ID,
			SourceID:       row.SourceID,
			SourceCode:     row.SourceCode,
			Cron:           row.Cron,
			LastEnqueuedAt: tsPtr(row.LastEnqueuedAt),
		})
	}
	return out, nil
}

func (r *Repository) TouchSchedule(ctx context.Context, id string) error {
	return r.q.TouchSchedule(ctx, id)
}

func (r *Repository) EnqueueJob(ctx context.Context, jobType, idempotencyKey, sourceID string, payload []byte, maxAttempts int) (string, string, error) {
	row, err := r.q.EnqueueJob(ctx, hermeneasql.EnqueueJobParams{
		JobType:        jobType,
		IdempotencyKey: idempotencyKey,
		SourceID:       text(sourceID),
		Payload:        payload,
		MaxAttempts:    int32(maxAttempts),
	})
	if err != nil {
		return "", "", err
	}
	return row.ID, row.Status, nil
}

func (r *Repository) ClaimJob(ctx context.Context, worker string) (domain.Job, bool, error) {
	row, err := r.q.ClaimJob(ctx, text(worker))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Job{}, false, nil
	}
	if err != nil {
		return domain.Job{}, false, err
	}
	return jobFrom(row), true, nil
}

func (r *Repository) MarkJobSucceeded(ctx context.Context, id string) error {
	return r.q.MarkJobSucceeded(ctx, id)
}

func (r *Repository) RescheduleJob(ctx context.Context, id string, runAfter time.Time, lastErr string) error {
	return r.q.RescheduleJob(ctx, hermeneasql.RescheduleJobParams{ID: id, RunAfter: ts(runAfter), LastError: text(lastErr)})
}

func (r *Repository) DeadLetterJob(ctx context.Context, id, lastErr string) error {
	return r.q.DeadLetterJob(ctx, hermeneasql.DeadLetterJobParams{ID: id, LastError: text(lastErr)})
}

func (r *Repository) ListJobs(ctx context.Context, limit int) ([]domain.Job, error) {
	rows, err := r.q.ListJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Job, 0, len(rows))
	for _, row := range rows {
		out = append(out, jobFrom(row))
	}
	return out, nil
}

func (r *Repository) UnhealthyJobs(ctx context.Context) (int, error) {
	n, err := r.q.CountUnhealthyJobs(ctx)
	return int(n), err
}

func (r *Repository) RequeueStaleRunning(ctx context.Context, lockedBefore time.Time) error {
	return r.q.RequeueStaleRunning(ctx, ts(lockedBefore))
}

func (r *Repository) SetJobCursor(ctx context.Context, id string, seq int, checksum string) error {
	return r.q.SetJobCursor(ctx, hermeneasql.SetJobCursorParams{
		ID:             id,
		ResumeSeq:      int64(seq),
		ResumeChecksum: text(checksum),
	})
}

func (r *Repository) InsertRawBatch(ctx context.Context, sourceID, sourceVersion, checksum string, payload []byte) (string, error) {
	return r.q.InsertRawBatch(ctx, hermeneasql.InsertRawBatchParams{
		SourceID:      sourceID,
		SourceVersion: text(sourceVersion),
		Checksum:      checksum,
		Payload:       payload,
	})
}

func (r *Repository) InsertRawBatchRef(ctx context.Context, sourceID, sourceVersion, checksum, stagedPath string) (string, error) {
	return r.q.InsertRawBatchRef(ctx, hermeneasql.InsertRawBatchRefParams{
		SourceID:      sourceID,
		SourceVersion: text(sourceVersion),
		Checksum:      checksum,
		StagedPath:    text(stagedPath),
	})
}

func (r *Repository) StartRun(ctx context.Context, sourceID, rawBatchID, sourceVersion string) (string, error) {
	return r.q.StartRun(ctx, hermeneasql.StartRunParams{
		SourceID:      sourceID,
		RawBatchID:    text(rawBatchID),
		SourceVersion: text(sourceVersion),
	})
}

func (r *Repository) FinishRun(ctx context.Context, id, status string, created, updated, skipped int, errMsg string) error {
	return r.q.FinishRun(ctx, hermeneasql.FinishRunParams{
		ID:           id,
		Status:       status,
		CreatedCount: int32(created),
		UpdatedCount: int32(updated),
		SkippedCount: int32(skipped),
		Error:        text(errMsg),
	})
}

func (r *Repository) ListRuns(ctx context.Context, limit int) ([]domain.Run, error) {
	rows, err := r.q.ListRuns(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Run{
			ID:            row.ID,
			SourceCode:    row.SourceID, // run rows reference source by id; code resolution is a future nicety
			SourceVersion: row.SourceVersion.String,
			Status:        row.Status,
			Created:       int(row.CreatedCount),
			Updated:       int(row.UpdatedCount),
			Skipped:       int(row.SkippedCount),
			Error:         row.Error.String,
			StartedAt:     row.StartedAt.Time,
			FinishedAt:    tsPtr(row.FinishedAt),
		})
	}
	return out, nil
}

// ---- mapping helpers ----

func sourceFrom(row hermeneasql.HermeneaImportSource) domain.Source {
	return domain.Source{
		ID:            row.ID,
		Code:          row.Code,
		Name:          row.Name,
		ConnectorType: row.ConnectorType,
		ObjectType:    row.ObjectType,
		Locator:       row.Locator,
		Cron:          row.Cron.String,
		Enabled:       row.Enabled,
	}
}

func jobFrom(row hermeneasql.HermeneaWorkerJob) domain.Job {
	var p jobPayload
	_ = json.Unmarshal(row.Payload, &p)
	return domain.Job{
		ID:             row.ID,
		JobType:        row.JobType,
		IdempotencyKey: row.IdempotencyKey,
		SourceCode:     p.Source,
		Status:         row.Status,
		Attempts:       int(row.Attempts),
		MaxAttempts:    int(row.MaxAttempts),
		RunAfter:       row.RunAfter.Time,
		LastError:      row.LastError.String,
		ResumeSeq:      int(row.ResumeSeq),
		ResumeChecksum: row.ResumeChecksum.String,
	}
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()} }

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
