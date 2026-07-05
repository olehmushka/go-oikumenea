// Package watchlistclient adapts the generated hermenea client to the person module's WatchlistLookup
// seam (D-Watchlists, M34). It is the oikumenea SIDE of the live screening call: the person service
// screens an identity by calling out to the hermenea companion (which owns the OFAC/EU/UN/INTERPOL
// egress + the ≤24h cache) and persists only the returned match metadata. Keeping this adapter out of
// the person package lets person's domain stay free of the hermenea client (the seam is late-bound in
// main.go, mirroring the location/color seams).
package watchlistclient

import (
	"context"
	"time"

	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
)

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
