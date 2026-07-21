// Package watchlistclient is the WATCHLIST lookup KIND of the connector plane's on-demand-lookup seam
// (D-Watchlists M34; generalized under D-ConnectorPlane M53). It adapts the generated hermenea client
// to the person module's WatchlistLookup interface: the person service screens an identity by calling
// out to the hermenea companion (which owns the OFAC/EU/UN/INTERPOL egress + the ≤24h cache) and
// persists only the returned match metadata. The transport discipline every such call needs — a
// mandatory deadline (R-12) and a null-object for "not configured" — lives in the shared
// internal/connectorcall seam; this package is the watchlist-specific mapping over a client that seam
// dials. Kept out of the person package so person's domain stays free of the hermenea client (the seam
// is late-bound in main.go, mirroring the location/color seams).
package watchlistclient

import (
	"context"
	"time"

	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/olegamysk/go-oikumenea/internal/connectorcall"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
)

// HTTPTimeout is the watchlist call's hard deadline (R-12). It is the shared connector-call deadline —
// aliased here so existing references and the M34 docs keep resolving; new lookup kinds use
// connectorcall.Deadline directly.
const HTTPTimeout = connectorcall.Deadline

// Client screens person identities against hermenea's watchlist endpoint. It holds the OIKUMENEA_HERMENEA
// trigger secret (the same trust direction as the import-control proxy) so the web tier never reaches the
// companion directly.
type Client struct {
	hc    hermeneaapi.HermeneaServiceClient
	token bearertoken.Token
}

// New builds the adapter over the generated hermenea client and the trigger secret.
func New(hc hermeneaapi.HermeneaServiceClient, token string) Client {
	return Client{hc: hc, token: bearertoken.Token(token)}
}

var _ persondomain.WatchlistLookup = Client{}

// Screen forwards the query to hermenea's POST /watchlist/check and maps the response to the person-domain
// screening result. Only match metadata crosses the boundary — never the lists.
func (c Client) Screen(ctx context.Context, q persondomain.WatchlistQuery) (persondomain.WatchlistScreenResult, error) {
	res, err := c.hc.CheckWatchlist(ctx, c.token, hermeneaapi.WatchlistQuery{
		SubjectKey:  q.SubjectKey,
		FullName:    q.FullName,
		Birthdate:   optStr(q.Birthdate),
		Nationality: optStr(q.Nationality),
	})
	if err != nil {
		return persondomain.WatchlistScreenResult{}, err
	}
	out := persondomain.WatchlistScreenResult{
		OnList:     res.OnList,
		Lists:      res.Lists,
		Program:    deref(res.Program),
		MatchScore: res.MatchScore,
	}
	if res.NextCheckDue != nil {
		if t, err := time.Parse(time.RFC3339, *res.NextCheckDue); err == nil {
			tu := t.UTC()
			out.NextCheckDue = &tu
		}
	}
	return out, nil
}

// Disabled is the explicit no-op WatchlistLookup bound when no hermenea companion is configured
// (review-2026-07 R-11). It makes the person service's seam always non-nil, so screening returns the
// clear "not configured" error via this implementation rather than a nil check in the service.
type Disabled struct{}

// Screen always reports the screening seam is unavailable.
func (Disabled) Screen(context.Context, persondomain.WatchlistQuery) (persondomain.WatchlistScreenResult, error) {
	return persondomain.WatchlistScreenResult{}, persondomain.ErrWatchlistUnavailable
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
