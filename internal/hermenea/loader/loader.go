// Package loader implements the hermenea Loader seam (M16 / D-Hermenea; chunked since R-05): it
// pushes canonical envelopes to oikumenea's public POST /import/{objectType} endpoint over HTTP —
// the ONLY oikumenea coupling (never the DB). It authenticates with the HERMENEA_OIKUMENEA_TOKEN
// service secret. Retry is OFF on the HTTP client (hermenea's own job queue owns retry/backoff), so
// one chunk POST = one attempt. A dataset larger than one chunk is sent as a chunked run — one
// envelope (= one oikumenea transaction) per ~ChunkSize records, tagged (runId, seq, isLast), ended
// by a trailing empty finalize chunk — so no single request carries a whole dataset and the client
// deadline is finite again (R-05 item 3).
package loader

import (
	"context"
	"time"

	dataimportapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/dataimport"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
	"github.com/palantir/pkg/bearertoken"
)

// Defaults for the chunk size and the per-request deadline (overridable via the install config).
const (
	DefaultChunkSize   = 5000
	DefaultHTTPTimeout = 120 * time.Second
)

// Options tunes the loader (zero values fall back to the defaults above).
type Options struct {
	ChunkSize   int           // records per envelope/transaction
	HTTPTimeout time.Duration // per-request client deadline (a request carries at most one chunk)
}

// Oikumenea is a domain.Loader backed by the generated ImportService client.
type Oikumenea struct {
	client    dataimportapi.ImportServiceClient
	token     bearertoken.Token
	chunkSize int
}

// New builds the loader against oikumenea's base URL with the import service secret. insecureTLS skips
// certificate verification (for the self-signed local-dev cert); set false against a real cert.
func New(baseURL, token string, insecureTLS bool, opts Options) (*Oikumenea, error) {
	if opts.ChunkSize <= 0 {
		opts.ChunkSize = DefaultChunkSize
	}
	if opts.HTTPTimeout <= 0 {
		opts.HTTPTimeout = DefaultHTTPTimeout
	}
	params := []httpclient.ClientParam{
		httpclient.WithBaseURLs([]string{baseURL}),
		httpclient.WithMaxRetries(0), // hermenea's queue owns retry/backoff
		// Finite deadline (R-05): a request carries at most one chunk, so the pre-chunking "no client
		// deadline, bounded only by worker.job-timeout-ms" escape hatch is gone. The trailing finalize
		// chunk (server-side closure rebuild) is the longest request and fits comfortably.
		httpclient.WithHTTPTimeout(opts.HTTPTimeout),
	}
	if insecureTLS {
		params = append(params, httpclient.WithTLSInsecureSkipVerify())
	}
	hc, err := httpclient.NewClient(params...)
	if err != nil {
		return nil, err
	}
	return &Oikumenea{
		client:    dataimportapi.NewImportServiceClient(hc),
		token:     bearertoken.Token(token),
		chunkSize: opts.ChunkSize,
	}, nil
}

var _ domain.Loader = (*Oikumenea)(nil)

// Load pushes a full in-memory record set. A set that fits one chunk goes as a single-shot envelope
// (no chunk fields — byte-identical to the pre-R-05 behavior for the small catalogs); anything larger
// becomes a chunked run.
func (l *Oikumenea) Load(ctx context.Context, objectType, source, sourceVersion, runID string, startSeq int, records []map[string]any, ack domain.AckFunc) (domain.ImportSummary, error) {
	if len(records) <= l.chunkSize && startSeq == 0 {
		return l.post(ctx, objectType, source, sourceVersion, nil)(records)
	}
	run := l.StartRun(objectType, source, sourceVersion, runID, startSeq, ack)
	if err := run.Push(ctx, records); err != nil {
		return domain.ImportSummary{}, err
	}
	return run.Finalize(ctx)
}

// StartRun opens a chunked run (streaming emitters push pages of any size; the run re-slices them
// into ≤chunkSize chunks with monotonically increasing seq, skipping those already acked).
func (l *Oikumenea) StartRun(objectType, source, sourceVersion, runID string, startSeq int, ack domain.AckFunc) domain.LoadRun {
	return &chunkedRun{
		loader:        l,
		objectType:    objectType,
		source:        source,
		sourceVersion: sourceVersion,
		runID:         runID,
		skipThrough:   startSeq,
		ack:           ack,
	}
}

// chunkedRun accumulates seq state across Pushes. It is NOT safe for concurrent use (one run = one
// sequential job).
type chunkedRun struct {
	loader        *Oikumenea
	objectType    string
	source        string
	sourceVersion string
	runID         string
	seq           int // last sent (or skipped) chunk seq
	skipThrough   int // resume cursor: chunks with seq <= skipThrough are not re-sent
	ack           domain.AckFunc
	agg           domain.ImportSummary
}

// Push re-slices records into chunks and sends each (skipping already-acked seqs on resume — the
// records are still mapped locally, just not re-posted).
func (r *chunkedRun) Push(ctx context.Context, records []map[string]any) error {
	for lo := 0; lo < len(records); lo += r.loader.chunkSize {
		hi := min(lo+r.loader.chunkSize, len(records))
		r.seq++
		if r.seq <= r.skipThrough {
			continue
		}
		if err := r.send(ctx, records[lo:hi], false); err != nil {
			return err
		}
	}
	return nil
}

// Finalize sends the trailing empty isLast chunk (running the object-type's batch finalizers
// server-side) and returns the aggregate summary of everything this attempt sent.
func (r *chunkedRun) Finalize(ctx context.Context) (domain.ImportSummary, error) {
	r.seq++
	if err := r.send(ctx, nil, true); err != nil {
		return domain.ImportSummary{}, err
	}
	return r.agg, nil
}

func (r *chunkedRun) send(ctx context.Context, records []map[string]any, isLast bool) error {
	seq := r.seq
	chunk := &dataimportapi.CanonicalEnvelope{}
	chunk.RunId = &r.runID
	chunk.Seq = &seq
	chunk.IsLast = &isLast
	sum, err := r.loader.post(ctx, r.objectType, r.source, r.sourceVersion, chunk)(records)
	if err != nil {
		return err
	}
	r.agg.Created += sum.Created
	r.agg.Updated += sum.Updated
	r.agg.Skipped += sum.Skipped
	if r.ack != nil {
		if err := r.ack(ctx, seq); err != nil {
			return err
		}
	}
	return nil
}

// post builds and sends one envelope; chunkFields (when non-nil) carries the pre-set runId/seq/isLast.
func (l *Oikumenea) post(ctx context.Context, objectType, source, sourceVersion string, chunkFields *dataimportapi.CanonicalEnvelope) func([]map[string]any) (domain.ImportSummary, error) {
	return func(records []map[string]any) (domain.ImportSummary, error) {
		env := dataimportapi.CanonicalEnvelope{
			ObjectType: objectType,
			Source:     source,
			Records:    toAny(records),
		}
		if sourceVersion != "" {
			env.SourceVersion = &sourceVersion
		}
		if chunkFields != nil {
			env.RunId = chunkFields.RunId
			env.Seq = chunkFields.Seq
			env.IsLast = chunkFields.IsLast
		}
		res, err := l.client.ImportObjects(ctx, l.token, objectType, env)
		if err != nil {
			return domain.ImportSummary{}, err
		}
		return domain.ImportSummary{Created: res.Created, Updated: res.Updated, Skipped: res.Skipped}, nil
	}
}

func toAny(records []map[string]any) []interface{} {
	out := make([]interface{}, 0, len(records))
	for _, r := range records {
		out = append(out, r)
	}
	return out
}
