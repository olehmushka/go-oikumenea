package transport

import (
	"context"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/palantir/pkg/bearertoken"
)

// HermeneaProxy adapts the generated HermeneaService server interface to a reverse proxy into the
// out-of-process hermenea companion (M16 / D-Hermenea). The browser/operator calls oikumenea's
// /hermenea/v1/* with their normal OIDC bearer; this gates on import.manage (PEP, same permission as
// the inbound import endpoint) and, when allowed, re-issues the call to the real hermenea using the
// OIKUMENEA_HERMENEA_TOKEN service secret. The trigger token thus lives only in oikumenea — the web
// tier never holds it and never reaches :9443 directly. Generated code in internal/conjure is never
// hand-edited.
type HermeneaProxy struct {
	pep    *pep.Enforcer
	client hermeneaapi.HermeneaServiceClient
	token  bearertoken.Token
}

// NewHermeneaProxy builds the proxy over the PEP enforcer, the generated hermenea client, and the
// runtime trigger secret.
func NewHermeneaProxy(enforcer *pep.Enforcer, client hermeneaapi.HermeneaServiceClient, token string) HermeneaProxy {
	return HermeneaProxy{pep: enforcer, client: client, token: bearertoken.Token(token)}
}

// compile-time assertion that the proxy satisfies the generated server interface.
var _ hermeneaapi.HermeneaService = HermeneaProxy{}

// authorize enforces import.manage on the caller's subject (instance-admin plane). The push trigger is
// a permission-sensitive operation, so an absent/other subject is denied.
func (p HermeneaProxy) authorize(ctx context.Context, token bearertoken.Token) error {
	return p.pep.RequireAnywhere(ctx, token, string(authzdomain.PermImportManage))
}

// TriggerSync enqueues a sync job for a source on the companion, after the import.manage check.
func (p HermeneaProxy) TriggerSync(ctx context.Context, authHeader bearertoken.Token, source string) (hermeneaapi.JobRef, error) {
	if err := p.authorize(ctx, authHeader); err != nil {
		return hermeneaapi.JobRef{}, err
	}
	return p.client.TriggerSync(ctx, p.token, source)
}

// ListSources lists the companion's registered import sources, after the import.manage check.
func (p HermeneaProxy) ListSources(ctx context.Context, authHeader bearertoken.Token) ([]hermeneaapi.ImportSource, error) {
	if err := p.authorize(ctx, authHeader); err != nil {
		return nil, err
	}
	return p.client.ListSources(ctx, p.token)
}

// ListRuns lists the companion's import-run lineage, after the import.manage check.
func (p HermeneaProxy) ListRuns(ctx context.Context, authHeader bearertoken.Token) ([]hermeneaapi.ImportRun, error) {
	if err := p.authorize(ctx, authHeader); err != nil {
		return nil, err
	}
	return p.client.ListRuns(ctx, p.token)
}

// ListJobs lists the companion's worker-job queue, after the import.manage check.
func (p HermeneaProxy) ListJobs(ctx context.Context, authHeader bearertoken.Token) ([]hermeneaapi.WorkerJob, error) {
	if err := p.authorize(ctx, authHeader); err != nil {
		return nil, err
	}
	return p.client.ListJobs(ctx, p.token)
}

// CheckWatchlist proxies a live watchlist screening check (D-Watchlists, M34) to the companion, after
// the import.manage check. In normal operation oikumenea's person module calls hermenea directly for a
// screening; this proxy route exposes the same for operators/diagnostics.
func (p HermeneaProxy) CheckWatchlist(ctx context.Context, authHeader bearertoken.Token, req hermeneaapi.WatchlistQuery) (hermeneaapi.WatchlistResult, error) {
	if err := p.authorize(ctx, authHeader); err != nil {
		return hermeneaapi.WatchlistResult{}, err
	}
	return p.client.CheckWatchlist(ctx, p.token, req)
}
