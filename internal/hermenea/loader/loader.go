// Package loader implements the hermenea Loader seam (M16 / D-Hermenea): it pushes a canonical
// envelope to oikumenea's public POST /import/{objectType} endpoint over HTTP — the ONLY oikumenea
// coupling (never the DB). It authenticates with the HERMENEA_OIKUMENEA_TOKEN service secret. Retry
// is OFF on the HTTP client (hermenea's own job queue owns retry/backoff), so one Load = one attempt.
package loader

import (
	"context"

	dataimportapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/dataimport"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
	"github.com/palantir/pkg/bearertoken"
)

// Oikumenea is a domain.Loader backed by the generated ImportService client.
type Oikumenea struct {
	client dataimportapi.ImportServiceClient
	token  bearertoken.Token
}

// New builds the loader against oikumenea's base URL with the import service secret. insecureTLS skips
// certificate verification (for the self-signed local-dev cert); set false against a real cert.
func New(baseURL, token string, insecureTLS bool) (*Oikumenea, error) {
	params := []httpclient.ClientParam{
		httpclient.WithBaseURLs([]string{baseURL}),
		httpclient.WithMaxRetries(0), // hermenea's queue owns retry/backoff
	}
	if insecureTLS {
		params = append(params, httpclient.WithTLSInsecureSkipVerify())
	}
	hc, err := httpclient.NewClient(params...)
	if err != nil {
		return nil, err
	}
	return &Oikumenea{client: dataimportapi.NewImportServiceClient(hc), token: bearertoken.Token(token)}, nil
}

var _ domain.Loader = (*Oikumenea)(nil)

// Load posts the envelope and returns oikumenea's idempotent upsert summary.
func (l *Oikumenea) Load(ctx context.Context, objectType, source, sourceVersion string, records []map[string]any) (domain.ImportSummary, error) {
	env := dataimportapi.CanonicalEnvelope{
		ObjectType: objectType,
		Source:     source,
		Records:    toAny(records),
	}
	if sourceVersion != "" {
		env.SourceVersion = &sourceVersion
	}
	res, err := l.client.ImportObjects(ctx, l.token, objectType, env)
	if err != nil {
		return domain.ImportSummary{}, err
	}
	return domain.ImportSummary{Created: res.Created, Updated: res.Updated, Skipped: res.Skipped}, nil
}

func toAny(records []map[string]any) []interface{} {
	out := make([]interface{}, 0, len(records))
	for _, r := range records {
		out = append(out, r)
	}
	return out
}
