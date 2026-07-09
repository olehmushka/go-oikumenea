// White-box tests for the chunked loader (R-05): chunk slicing, seq numbering, the trailing finalize
// chunk, single-shot passthrough for small sets, resume skip-ahead, and per-chunk acks — against a
// fake ImportService client (no HTTP).
package loader

import (
	"context"
	"testing"

	dataimportapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/dataimport"
	"github.com/palantir/pkg/bearertoken"
)

// sentEnvelope is one recorded ImportObjects call, flattened for assertions.
type sentEnvelope struct {
	records int
	runID   string // "" = field absent
	seq     int    // 0 = field absent
	isLast  *bool  // nil = field absent
}

type fakeClient struct{ sent []sentEnvelope }

func (f *fakeClient) ImportObjects(_ context.Context, _ bearertoken.Token, _ string, env dataimportapi.CanonicalEnvelope) (dataimportapi.ImportResult, error) {
	e := sentEnvelope{records: len(env.Records)}
	if env.RunId != nil {
		e.runID = *env.RunId
	}
	if env.Seq != nil {
		e.seq = *env.Seq
	}
	e.isLast = env.IsLast
	f.sent = append(f.sent, e)
	return dataimportapi.ImportResult{Created: len(env.Records)}, nil
}

func newTestLoader(chunkSize int) (*Oikumenea, *fakeClient) {
	fc := &fakeClient{}
	return &Oikumenea{client: fc, chunkSize: chunkSize}, fc
}

func recs(n int) []map[string]any {
	out := make([]map[string]any, n)
	for i := range out {
		out[i] = map[string]any{"i": i}
	}
	return out
}

// TestLoadSingleShot: a set that fits one chunk goes as ONE envelope without chunk fields — the
// pre-R-05 behavior for small catalogs (touched-gated finalizers server-side).
func TestLoadSingleShot(t *testing.T) {
	l, fc := newTestLoader(5)
	sum, err := l.Load(context.Background(), "colors", "src", "v1", "run-1", 0, recs(5), nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(fc.sent) != 1 {
		t.Fatalf("envelopes = %d, want 1", len(fc.sent))
	}
	e := fc.sent[0]
	if e.records != 5 || e.runID != "" || e.seq != 0 || e.isLast != nil {
		t.Fatalf("single-shot envelope carried chunk fields: %+v", e)
	}
	if sum.Created != 5 {
		t.Fatalf("summary = %+v, want Created=5", sum)
	}
}

// TestLoadChunked: a larger set becomes a chunked run — sequential seqs, bounded chunks, and a
// trailing empty isLast finalize chunk; every acked seq is reported in order.
func TestLoadChunked(t *testing.T) {
	l, fc := newTestLoader(5)
	var acked []int
	ack := func(_ context.Context, seq int) error { acked = append(acked, seq); return nil }
	sum, err := l.Load(context.Background(), "geo-places", "src", "v1", "run-2", 0, recs(12), ack)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	wantSizes := []int{5, 5, 2, 0}
	if len(fc.sent) != len(wantSizes) {
		t.Fatalf("envelopes = %d, want %d", len(fc.sent), len(wantSizes))
	}
	for i, e := range fc.sent {
		if e.records != wantSizes[i] || e.seq != i+1 || e.runID != "run-2" {
			t.Fatalf("chunk %d = %+v, want records=%d seq=%d", i, e, wantSizes[i], i+1)
		}
		wantLast := i == len(wantSizes)-1
		if e.isLast == nil || *e.isLast != wantLast {
			t.Fatalf("chunk %d isLast = %v, want %v", i, e.isLast, wantLast)
		}
	}
	if len(acked) != 4 || acked[0] != 1 || acked[3] != 4 {
		t.Fatalf("acked = %v, want [1 2 3 4]", acked)
	}
	if sum.Created != 12 {
		t.Fatalf("summary = %+v, want Created=12 (finalize chunk adds nothing)", sum)
	}
}

// TestLoadResume: startSeq=2 skips the first two chunks (already acked by a previous attempt) and
// re-sends only the remainder + finalize.
func TestLoadResume(t *testing.T) {
	l, fc := newTestLoader(5)
	var acked []int
	ack := func(_ context.Context, seq int) error { acked = append(acked, seq); return nil }
	sum, err := l.Load(context.Background(), "geo-places", "src", "v1", "run-3", 2, recs(12), ack)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(fc.sent) != 2 {
		t.Fatalf("envelopes = %d, want 2 (chunk 3 + finalize)", len(fc.sent))
	}
	if fc.sent[0].seq != 3 || fc.sent[0].records != 2 {
		t.Fatalf("resumed chunk = %+v, want seq=3 records=2", fc.sent[0])
	}
	if fc.sent[1].seq != 4 || fc.sent[1].records != 0 || fc.sent[1].isLast == nil || !*fc.sent[1].isLast {
		t.Fatalf("finalize chunk = %+v, want seq=4 empty isLast", fc.sent[1])
	}
	if len(acked) != 2 || acked[0] != 3 || acked[1] != 4 {
		t.Fatalf("acked = %v, want [3 4]", acked)
	}
	if sum.Created != 2 {
		t.Fatalf("summary = %+v, want Created=2 (only what this attempt sent)", sum)
	}
}

// TestStartRunPages: streamed pages are re-sliced per push (no cross-push buffering — page boundaries
// stay deterministic for the resume cursor), with one continuous seq across pushes.
func TestStartRunPages(t *testing.T) {
	l, fc := newTestLoader(5)
	run := l.StartRun("geo-places", "src", "v1", "run-4", 0, nil)
	if err := run.Push(context.Background(), recs(7)); err != nil {
		t.Fatalf("push 1: %v", err)
	}
	if err := run.Push(context.Background(), recs(4)); err != nil {
		t.Fatalf("push 2: %v", err)
	}
	sum, err := run.Finalize(context.Background())
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	wantSizes := []int{5, 2, 4, 0} // push1 → 5+2, push2 → 4, finalize → 0
	if len(fc.sent) != len(wantSizes) {
		t.Fatalf("envelopes = %d, want %d", len(fc.sent), len(wantSizes))
	}
	for i, e := range fc.sent {
		if e.records != wantSizes[i] || e.seq != i+1 {
			t.Fatalf("chunk %d = %+v, want records=%d seq=%d", i, e, wantSizes[i], i+1)
		}
	}
	if sum.Created != 11 {
		t.Fatalf("summary = %+v, want Created=11", sum)
	}
}
