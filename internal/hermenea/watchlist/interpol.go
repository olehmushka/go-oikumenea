// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package watchlist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
	"github.com/olehmushka/go-oikumenea/internal/hermenea/fetcher"
)

// InterpolBaseURL is INTERPOL's public Red Notices endpoint (documented at interpol.api.bund.dev, the
// original todo.md item 1 / D-Watchlists first live-lookup connector). Overridable for tests.
const InterpolBaseURL = "https://ws-public.interpol.int/notices/v1/red"

// Interpol screens a name against INTERPOL's public Red Notices list. It is the real live-lookup
// connector (D-Watchlists): a non-empty result set is a hit on INTERPOL_RED. Egress lives here in the
// companion — oikumenea's PDP core never calls out.
type Interpol struct {
	baseURL string
	client  *http.Client
}

// NewInterpol builds the provider. An empty baseURL falls back to the public endpoint; a nil client
// gets a 15s default.
func NewInterpol(baseURL string, client *http.Client) Interpol {
	if baseURL == "" {
		baseURL = InterpolBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return Interpol{baseURL: baseURL, client: client}
}

func (Interpol) Name() string { return "interpol-red" }

// interpolResponse is the subset of the Red Notices response we consume.
type interpolResponse struct {
	Total    int `json:"total"`
	Embedded struct {
		Notices []struct {
			Forename string `json:"forename"`
			Name     string `json:"name"`
		} `json:"notices"`
	} `json:"_embedded"`
}

// Screen queries the Red Notices endpoint by surname (+ forename when derivable). A non-empty result set
// is a hit; the score is 1.0 on an exact case-insensitive full-name match among the results, else 0.6.
func (i Interpol) Screen(ctx context.Context, q domain.WatchlistQuery) (Hit, error) {
	forename, surname := splitName(q.FullName)
	if surname == "" {
		return Hit{}, nil
	}
	params := url.Values{}
	params.Set("name", surname)
	if forename != "" {
		params.Set("forename", forename)
	}
	params.Set("resultPerPage", "20")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return Hit{}, err
	}
	req.Header.Set("User-Agent", fetcher.UserAgent())
	req.Header.Set("Accept", "application/json")
	resp, err := i.client.Do(req)
	if err != nil {
		return Hit{}, fmt.Errorf("interpol screen: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Hit{}, fmt.Errorf("interpol screen: %s returned %d", i.baseURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Hit{}, err
	}
	var out interpolResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return Hit{}, fmt.Errorf("interpol screen: decode: %w", err)
	}
	if out.Total == 0 && len(out.Embedded.Notices) == 0 {
		return Hit{}, nil
	}

	score := 0.6
	want := strings.ToLower(strings.TrimSpace(q.FullName))
	for _, n := range out.Embedded.Notices {
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
