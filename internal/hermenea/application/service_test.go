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
	src       domain.Source
	finished  bool
	finStatus string
	created   int
	updated   int
	skipped   int
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

// fakeStreamingConnector stages a no-op source.
type fakeStreamingConnector struct{ cleaned *bool }

func (fakeStreamingConnector) Fetch(context.Context, domain.Source) (domain.RawBatch, error) {
	return domain.RawBatch{}, domain.ErrNoConnector
}
func (c fakeStreamingConnector) Stage(context.Context, domain.Source) (domain.StagedSource, error) {
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

// fakeLoader returns a fixed per-page summary so aggregation is observable.
type fakeLoader struct{ calls int }

func (l *fakeLoader) Load(_ context.Context, _, _, _ string, recs []map[string]any) (domain.ImportSummary, error) {
	l.calls++
	return domain.ImportSummary{Created: len(recs)}, nil // page1 -> 2, page2 -> 1
}

// TestProcessJob_Streaming verifies the wof-sqlite/paged path stages once, loads each page, aggregates
// the counts into one succeeded run, and cleans up the staged file.
func TestProcessJob_Streaming(t *testing.T) {
	cleaned := false
	store := &fakeStore{src: domain.Source{
		ID: "s1", Code: "wof-geo-ua", ConnectorType: domain.ConnectorWOFSQLite, ObjectType: "geo-places",
	}}
	loader := &fakeLoader{}
	svc := NewService(store, map[string]domain.Connector{
		domain.ConnectorWOFSQLite: fakeStreamingConnector{cleaned: &cleaned},
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
