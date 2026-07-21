// Package reporter is hermenea's connector-plane REPORTING seam (M53 / D-ConnectorPlane): the thin
// outbound adapter by which hermenea makes itself VISIBLE in oikumenea's connector registry. It posts
// hermenea's registry row + declared sources at boot (idempotent self-registration) and reports each
// sync run's open/close, so an operator sees the fleet from the core where they already look.
//
// It is a SECOND coupling to oikumenea beside the loader, and it reuses the loader's exact trust seam:
// the HERMENEA_OIKUMENEA_TOKEN shared secret, which oikumenea's authenticator resolves to the
// `hermenea-importer` service principal (M51 / D-ServiceIdentities). So the same token that authorizes
// pushes authorizes reporting; hermenea needs no OIDC client-credentials flow of its own. Retry is OFF
// (reporting is best-effort — a report failure never fails the underlying job), so one call = one
// attempt.
//
// Direction of ownership: hermenea stays authoritative for execution; the core registry is a READ
// model of what hermenea reports (visibility, not orchestration). Reporter implements the
// domain.RunReporter port the application service calls; module.go calls Register directly at boot.
package reporter

import (
	"context"
	"time"

	connectorapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/connector"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// DefaultHTTPTimeout bounds a single report call. Reporting is off the import hot path (it opens/closes
// a run, carrying no dataset), so the deadline is short — a slow or absent core must not stall a job.
const DefaultHTTPTimeout = 10 * time.Second

// Reporter posts to the core connector registry over the SAME base URL and shared secret the loader
// uses. It implements domain.RunReporter.
type Reporter struct {
	client connectorapi.ConnectorServiceClient
	token  bearertoken.Token
}

var _ domain.RunReporter = (*Reporter)(nil)

// New builds a Reporter against oikumenea's base URL with the import shared secret. insecureTLS skips
// certificate verification (self-signed local-dev cert); set false against a real cert. A retry-free,
// deadline-bounded client (best-effort reporting).
func New(baseURL, token string, insecureTLS bool, httpTimeout time.Duration) (*Reporter, error) {
	if httpTimeout <= 0 {
		httpTimeout = DefaultHTTPTimeout
	}
	params := []httpclient.ClientParam{
		httpclient.WithBaseURLs([]string{baseURL}),
		httpclient.WithMaxRetries(0), // reporting is best-effort; one call = one attempt
		httpclient.WithHTTPTimeout(httpTimeout),
	}
	if insecureTLS {
		params = append(params, httpclient.WithTLSInsecureSkipVerify())
	}
	hc, err := httpclient.NewClient(params...)
	if err != nil {
		return nil, err
	}
	return &Reporter{
		client: connectorapi.NewConnectorServiceClient(hc),
		token:  bearertoken.Token(token),
	}, nil
}

// Register self-registers this connector and REPLACES its declared source set (the core retires sources
// absent from the payload), so a boot-time call converges the registry. The core binds the row to the
// calling principal — this request names no principal, by design. A nil *Reporter is a no-op.
func (r *Reporter) Register(ctx context.Context, code, name, description string, sources []domain.Source) error {
	if r == nil {
		return nil
	}
	decls := make([]connectorapi.SourceDeclaration, 0, len(sources))
	for _, s := range sources {
		d := connectorapi.SourceDeclaration{Code: s.Code, Name: s.Name}
		if s.ObjectType != "" {
			ot := s.ObjectType
			d.ObjectType = &ot
		}
		if s.Cron != "" {
			sc := s.Cron
			d.Schedule = &sc
		}
		en := s.Enabled
		d.Enabled = &en
		decls = append(decls, d)
	}
	req := connectorapi.RegisterConnectorRequest{Code: code, Name: name, Sources: decls}
	if description != "" {
		req.Description = &description
	}
	_, err := r.client.RegisterConnector(ctx, r.token, req)
	return err
}

// ReportRun opens (state=running) or closes (succeeded/failed) a run for a source of THIS connector,
// satisfying domain.RunReporter. Idempotent on (source, externalRunID) core-side, so a re-report
// updates rather than duplicates. sum/errMsg are meaningful on close; on open they are zero/empty. A
// nil *Reporter is a no-op, so a service built without a reporter skips reporting cleanly.
func (r *Reporter) ReportRun(ctx context.Context, sourceCode, externalRunID, state string, sum domain.ImportSummary, errMsg string) error {
	if r == nil {
		return nil
	}
	req := connectorapi.ReportSyncRunRequest{SourceCode: sourceCode, State: state}
	if externalRunID != "" {
		req.ExternalRunId = &externalRunID
	}
	c, u, s := sum.Created, sum.Updated, sum.Skipped
	req.Created, req.Updated, req.Skipped = &c, &u, &s
	if errMsg != "" {
		req.Error = &errMsg
	}
	// A terminal report MUST carry finishedAt (the core rejects a finished run without it, 400); a
	// running one must NOT. finishRun calls this the instant the run ends, so now() is the finish time.
	if state == domain.RunSucceeded || state == domain.RunFailed {
		ft := datetime.DateTime(time.Now().UTC())
		req.FinishedAt = &ft
	}
	_, err := r.client.ReportSyncRun(ctx, r.token, req)
	return err
}
