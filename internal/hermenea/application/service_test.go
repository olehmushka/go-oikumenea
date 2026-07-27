// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// fakeStore embeds domain.Store so the test implements only the methods the streaming path touches;
// any unimplemented call would panic (and would signal the path changed).
type fakeStore struct {
	domain.Store
	src            domain.Source
	finished       bool
	finStatus      string
	created        int
	updated        int
	skipped        int
	cursors        []int
	cursorChecksum string
}

func (f *fakeStore) SetJobCursor(_ context.Context, _ string, seq int, checksum string) error {
	f.cursors = append(f.cursors, seq)
	f.cursorChecksum = checksum
	return nil
}

func (f *fakeStore) GetSourceByCode(context.Context, string) (domain.Source, bool, error) {
	return f.src, true, nil
}
func (f *fakeStore) InsertRawBatchRef(context.Context, string, string, string, string) (string, error) {
	return "raw-1", nil
}
func (f *fakeStore) StartRun(context.Context, string, string, string) (string, error) {
	return "run-1", nil
}
func (f *fakeStore) FinishRun(_ context.Context, _, status string, c, u, s int, _ string) error {
	f.finished, f.finStatus, f.created, f.updated, f.skipped = true, status, c, u, s
	return nil
}

// fakeStreamingFetcher stages a no-op source.
type fakeStreamingFetcher struct{ cleaned *bool }

func (fakeStreamingFetcher) Fetch(context.Context, domain.Source) (domain.RawBatch, error) {
	return domain.RawBatch{}, domain.ErrNoFetcher
}
func (c fakeStreamingFetcher) Stage(context.Context, domain.Source) (domain.StagedSource, error) {
	return domain.StagedSource{Path: "/tmp/x.db", SourceVersion: "v1", Checksum: "abc", Cleanup: func() { *c.cleaned = true }}, nil
}

// twoPageMapper emits two pages; the pipeline must aggregate their load summaries into one run.
type twoPageMapper struct{}

func (twoPageMapper) MapPaged(_ context.Context, _ domain.StagedSource, emit domain.PageFunc) error {
	if err := emit([]map[string]any{{"wofId": 1}, {"wofId": 2}}); err != nil {
		return err
	}
	return emit([]map[string]any{{"wofId": 3}})
}

// fakeLoader returns a fixed per-push summary so aggregation is observable, records the startSeq the
// pipeline resumed from, and acks one seq per push (so the cursor wiring to the store is visible).
type fakeLoader struct {
	calls    int
	startSeq int
}

func (l *fakeLoader) Load(_ context.Context, _, _, _, _ string, startSeq int, recs []map[string]any, _ domain.AckFunc) (domain.ImportSummary, error) {
	l.calls++
	l.startSeq = startSeq
	return domain.ImportSummary{Created: len(recs)}, nil
}

func (l *fakeLoader) StartRun(_, _, _, _ string, startSeq int, ack domain.AckFunc) domain.LoadRun {
	l.startSeq = startSeq
	return &fakeRun{l: l, seq: startSeq, ack: ack}
}

// fakeRun counts pushes as loader calls and aggregates created=len(records) per push (page1 -> 2,
// page2 -> 1), mirroring what the old per-page Load returned; each push acks the next seq.
type fakeRun struct {
	l   *fakeLoader
	seq int
	ack domain.AckFunc
	agg domain.ImportSummary
}

func (r *fakeRun) Push(ctx context.Context, recs []map[string]any) error {
	r.l.calls++
	r.seq++
	r.agg.Created += len(recs)
	if r.ack != nil {
		return r.ack(ctx, r.seq)
	}
	return nil
}
func (r *fakeRun) Finalize(context.Context) (domain.ImportSummary, error) { return r.agg, nil }

// TestProcessJob_Streaming verifies the wof-sqlite/paged path stages once, loads each page, aggregates
// the counts into one succeeded run, and cleans up the staged file.
func TestProcessJob_Streaming(t *testing.T) {
	cleaned := false
	store := &fakeStore{src: domain.Source{
		ID: "s1", Code: "wof-geo-ua", FetcherType: domain.FetcherWOFSQLite, ObjectType: "geo-places",
	}}
	loader := &fakeLoader{}
	svc := NewService(store, map[string]domain.Fetcher{
		domain.FetcherWOFSQLite: fakeStreamingFetcher{cleaned: &cleaned},
	}, loader)
	svc.RegisterPagedMapper("geo-places", twoPageMapper{})

	err := svc.ProcessJob(context.Background(), domain.Job{JobType: domain.JobSync, SourceCode: "wof-geo-ua"})
	if err != nil {
		t.Fatalf("ProcessJob: %v", err)
	}
	if loader.calls != 2 {
		t.Fatalf("loader calls = %d, want 2 (one per page)", loader.calls)
	}
	if !store.finished || store.finStatus != domain.RunSucceeded {
		t.Fatalf("run not finished succeeded: %+v", store)
	}
	if store.created != 3 { // 2 + 1 aggregated across pages
		t.Fatalf("aggregated created = %d, want 3", store.created)
	}
	if !cleaned {
		t.Fatal("staged file was not cleaned up")
	}
}

// TestResumeSeqChecksumGuard: the persisted cursor only applies while the re-staged source still
// carries the checksum it was written against; anything else resets to a full re-run.
func TestResumeSeqChecksumGuard(t *testing.T) {
	s := &Service{}
	job := domain.Job{ResumeSeq: 7, ResumeChecksum: "abc"}
	if got := s.resumeSeq(job, "abc"); got != 7 {
		t.Fatalf("matching checksum: resume = %d, want 7", got)
	}
	if got := s.resumeSeq(job, "other"); got != 0 {
		t.Fatalf("changed checksum: resume = %d, want 0", got)
	}
	if got := s.resumeSeq(domain.Job{}, "abc"); got != 0 {
		t.Fatalf("no cursor: resume = %d, want 0", got)
	}
}

// TestProcessJob_StreamingResume: a retried job whose cursor checksum matches the re-staged source
// resumes the chunked run from that seq, and every acked chunk persists the cursor via the store.
func TestProcessJob_StreamingResume(t *testing.T) {
	cleaned := false
	store := &fakeStore{src: domain.Source{
		ID: "s1", Code: "wof-geo-ua", FetcherType: domain.FetcherWOFSQLite, ObjectType: "geo-places",
	}}
	loader := &fakeLoader{}
	svc := NewService(store, map[string]domain.Fetcher{
		domain.FetcherWOFSQLite: fakeStreamingFetcher{cleaned: &cleaned},
	}, loader)
	svc.RegisterPagedMapper("geo-places", twoPageMapper{})

	// The fake connector stages with Checksum "abc"; the job's cursor was written against it.
	job := domain.Job{ID: "job-1", JobType: domain.JobSync, SourceCode: "wof-geo-ua", ResumeSeq: 5, ResumeChecksum: "abc"}
	if err := svc.ProcessJob(context.Background(), job); err != nil {
		t.Fatalf("ProcessJob: %v", err)
	}
	if loader.startSeq != 5 {
		t.Fatalf("loader startSeq = %d, want 5 (resumed)", loader.startSeq)
	}
	if len(store.cursors) != 2 || store.cursors[0] != 6 || store.cursors[1] != 7 {
		t.Fatalf("acked cursors = %v, want [6 7]", store.cursors)
	}
	if store.cursorChecksum != "abc" {
		t.Fatalf("cursor checksum = %q, want abc", store.cursorChecksum)
	}

	// A different cursor checksum (source changed between attempts) resets to a fresh run.
	store2 := &fakeStore{src: store.src}
	loader2 := &fakeLoader{}
	svc2 := NewService(store2, map[string]domain.Fetcher{
		domain.FetcherWOFSQLite: fakeStreamingFetcher{cleaned: &cleaned},
	}, loader2)
	svc2.RegisterPagedMapper("geo-places", twoPageMapper{})
	job2 := domain.Job{ID: "job-2", JobType: domain.JobSync, SourceCode: "wof-geo-ua", ResumeSeq: 5, ResumeChecksum: "stale"}
	if err := svc2.ProcessJob(context.Background(), job2); err != nil {
		t.Fatalf("ProcessJob (stale checksum): %v", err)
	}
	if loader2.startSeq != 0 {
		t.Fatalf("stale-checksum startSeq = %d, want 0 (full re-run)", loader2.startSeq)
	}
}
