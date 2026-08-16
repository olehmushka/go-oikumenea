// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package watchlist

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	interpolclient "github.com/olehmushka/go-interpol-client"
	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
	"github.com/olehmushka/go-oikumenea/internal/hermenea/fetcher"
)

// InterpolBaseURL is INTERPOL's public Red Notices endpoint (documented at interpol.api.bund.dev, the
// original todo.md item 1 / D-Watchlists first live-lookup connector). Overridable for tests.
const InterpolBaseURL = interpolclient.DefaultBaseURL

// Interpol screens a name against INTERPOL's public Red Notices list. It is the real live-lookup
// connector (D-Watchlists): a non-empty result set is a hit on INTERPOL_RED. Egress lives here in the
// companion — oikumenea's PDP core never calls out. The transport layer (the actual API call) is
// go-interpol-client, extracted from this file; the score heuristic and Hit-building below stay
// here — hermenea's own screening business logic, not something a generic API client should decide.
type Interpol struct {
	client *interpolclient.Client
}

// NewInterpol builds the provider. An empty baseURL falls back to the public endpoint; a nil client
// gets go-interpol-client's own 15s default.
func NewInterpol(baseURL string, httpClient *http.Client) Interpol {
	opts := []interpolclient.Option{interpolclient.WithUserAgent(fetcher.UserAgent())}
	if baseURL != "" {
		opts = append(opts, interpolclient.WithBaseURL(baseURL))
	}
	return Interpol{client: interpolclient.New(httpClient, opts...)}
}

func (Interpol) Name() string { return "interpol-red" }

// Screen queries the Red Notices endpoint by surname (+ forename when derivable). A non-empty result set
// is a hit; the score is 1.0 on an exact case-insensitive full-name match among the results, else 0.6.
func (i Interpol) Screen(ctx context.Context, q domain.WatchlistQuery) (Hit, error) {
	forename, surname := splitName(q.FullName)
	if surname == "" {
		return Hit{}, nil
	}

	out, err := i.client.SearchRedNotices(ctx, interpolclient.Query{Surname: surname, Forename: forename})
	if err != nil {
		return Hit{}, fmt.Errorf("interpol screen: %w", err)
	}
	if out.Total == 0 && len(out.Notices) == 0 {
		return Hit{}, nil
	}

	score := 0.6
	want := strings.ToLower(strings.TrimSpace(q.FullName))
	for _, n := range out.Notices {
		full := strings.ToLower(strings.TrimSpace(n.Forename + " " + n.Name))
		if full == want {
			score = 1.0
			break
		}
	}
	return Hit{OnList: true, Lists: []string{"INTERPOL_RED"}, Program: "INTERPOL Red Notice", Score: &score}, nil
}

// splitName derives (forename, surname) from a full name: the last whitespace token is the surname, the
// rest the forename. A single token is treated as the surname (the Red Notices `name` field).
func splitName(full string) (forename, surname string) {
	fields := strings.Fields(strings.TrimSpace(full))
	switch len(fields) {
	case 0:
		return "", ""
	case 1:
		return "", fields[0]
	default:
		return strings.Join(fields[:len(fields)-1], " "), fields[len(fields)-1]
	}
}

// SanctionsStub is the documented pluggable extension point for OFAC/EU/UN sanctions screening. It
// returns no match unless a real provider is configured — the default deployment screens INTERPOL only
// (D-Watchlists: sanctions providers are a stub). A real implementation replaces this behind the same
// Provider interface with no call-site change.
type SanctionsStub struct{}

func (SanctionsStub) Name() string { return "sanctions-stub" }

func (SanctionsStub) Screen(context.Context, domain.WatchlistQuery) (Hit, error) { return Hit{}, nil }
