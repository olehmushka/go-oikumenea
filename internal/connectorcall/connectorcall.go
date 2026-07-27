// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package connectorcall is the ON-DEMAND-LOOKUP seam of the connector plane (M53 / D-ConnectorPlane):
// the one place oikumenea makes SYNCHRONOUS calls OUT to a connector. It owns the two invariants every
// such call must have, so no individual call site can forget them:
//
//   - a MANDATORY deadline (review-2026-07 R-12): a hung connector must fail into the caller's
//     "unavailable" path rather than couple oikumenea's request latency (and, before R-03, a pooled DB
//     connection) to a third party's tail latency. The deadline is applied by Dial, not passed by the
//     caller, so it cannot be omitted or set to zero.
//   - a NULL-OBJECT discipline (review-2026-07 R-11): a lookup kind that is not configured is an
//     explicit "disabled" implementation returning a clear error, never a nil seam the caller must
//     nil-check.
//
// Each lookup KIND is a per-kind interface owned by its consumer's domain (e.g. person's
// WatchlistLookup), late-bound in main.go over a client this package dials. The M34 watchlist check is
// the first kind; a second kind reuses Dial and follows the same disabled-implementation convention
// rather than re-deriving the transport discipline. Binding a new kind is still a line in main.go (the
// seam is late-bound by design) — this package removes the boilerplate, not that line.
package connectorcall

import (
	"time"

	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
	werror "github.com/palantir/witchcraft-go-error"
)

// Deadline is the hard per-call deadline on every synchronous call OUT to a connector (R-12). A
// connector answers cache hits in milliseconds; a miss that needs longer than this fails into the
// caller's unavailable path. It is not configurable per call — that is the point.
const Deadline = 10 * time.Second

// Dial builds a deadline-bounded HTTP client to a connector's base URL. The deadline (Deadline) and a
// no-retry policy are applied HERE so a call site cannot forget them — the R-12 discipline the M34
// watchlist client established, now enforced for every lookup kind. `insecureSkipVerify` trusts a
// self-signed dev cert (never in production).
func Dial(baseURL string, insecureSkipVerify bool) (httpclient.Client, error) {
	if baseURL == "" {
		return nil, werror.Error("connectorcall.Dial: empty base URL")
	}
	params := []httpclient.ClientParam{
		httpclient.WithBaseURLs([]string{baseURL}),
		// No retries: a synchronous lookup that fails should surface at once, not multiply the tail
		// latency the deadline exists to bound.
		httpclient.WithMaxRetries(0),
		httpclient.WithHTTPTimeout(Deadline),
	}
	if insecureSkipVerify {
		params = append(params, httpclient.WithTLSInsecureSkipVerify())
	}
	c, err := httpclient.NewClient(params...)
	if err != nil {
		return nil, werror.Wrap(err, "connectorcall.Dial: build client")
	}
	return c, nil
}
