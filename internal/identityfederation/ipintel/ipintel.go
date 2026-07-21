// Package ipintel is the IP-intelligence seam for the login security log (M37 / D-LoginSecurityLog):
// given a client IP, resolve country / ISP / VPN / Tor. A real resolver needs an offline data source
// (a GeoIP database, a Tor exit-node list, a VPN-range set) and is DEFERRED — the natural future shape
// is a hermenea connector (like the watchlist / factbook seams). The shipped default is a no-op that
// returns an empty overlay, so the MVP records raw ip + user_agent with the resolved_* columns NULL.
package ipintel

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
)

// Resolver turns a client IP into an IPIntel overlay. Implementations MUST be best-effort and cheap:
// the login recorder calls this off the request's critical path, and a failure must degrade to an
// empty overlay, never propagate.
type Resolver interface {
	Resolve(ctx context.Context, ip string) domain.IPIntel
}

// NoOp is the default resolver: every field nil. Deterministic, dependency-free, always available.
type NoOp struct{}

// Resolve returns an empty overlay.
func (NoOp) Resolve(ctx context.Context, ip string) domain.IPIntel { return domain.IPIntel{} }
