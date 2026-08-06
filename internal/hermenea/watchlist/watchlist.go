// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package watchlist implements hermenea's live watchlist screening (D-Watchlists, M34): the first
// SYNCHRONOUS oikumenea→hermenea surface. hermenea owns the outbound egress to the screening providers
// (OFAC/EU/UN/INTERPOL) and a short-TTL (≤24h) cache; only per-person MATCH METADATA is returned —
// never the lists themselves. A check is cache-first: a fresh cache row short-circuits the provider
// fan-out. The real interpol.api.bund.dev connector ships; sanctions providers are a documented stub.
package watchlist

import (
	"context"
	"encoding/json"
	"time"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// Hit is one provider's partial screening outcome for a query. The Service aggregates hits across
// providers into a single domain.WatchlistResult.
type Hit struct {
	OnList  bool
	Lists   []string // list codes matched, e.g. {INTERPOL_RED}
	Program string
	Score   *float64 // 0..1 best-match score for this provider, when available
}

// Provider screens a person-identity query against one external source. A provider returns a zero Hit
// (OnList=false) on no match; a transport error is returned so the Service can decide to fail the check
// rather than record a false negative.
type Provider interface {
	// Name identifies the provider in logs/errors (not persisted).
	Name() string
	Screen(ctx context.Context, q domain.WatchlistQuery) (Hit, error)
}

// Cache is the ≤24h short-TTL store for screening results (hermenea.watchlist_cache). A fresh entry
// short-circuits the provider fan-out.
type Cache interface {
	Get(ctx context.Context, subjectKey string) (domain.WatchlistResult, bool, error)
	Put(ctx context.Context, subjectKey string, r domain.WatchlistResult) error
}

// Service fans a query out across its providers (after a cache check) and aggregates the hits. It
// implements domain.WatchlistChecker.
type Service struct {
	providers []Provider
	cache     Cache
	ttl       time.Duration
	now       func() time.Time
}

// DefaultTTL is the screening cache lifetime — the D-Watchlists ≤24h bound.
const DefaultTTL = 24 * time.Hour

// NewService builds the checker over its providers + cache + TTL. A zero ttl falls back to DefaultTTL.
func NewService(providers []Provider, cache Cache, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Service{providers: providers, cache: cache, ttl: ttl, now: time.Now}
}

var _ domain.WatchlistChecker = (*Service)(nil)

// Check runs a screening check for the query. It first consults the cache; on a fresh hit it returns the
// cached metadata, otherwise it screens every provider, aggregates, stores, and returns. A provider
// transport error fails the whole check (better than persisting a false "clear").
func (s *Service) Check(ctx context.Context, q domain.WatchlistQuery) (domain.WatchlistResult, error) {
	now := s.now().UTC()
	if s.cache != nil && q.SubjectKey != "" {
		if cached, ok, err := s.cache.Get(ctx, q.SubjectKey); err == nil && ok {
			if cached.NextCheckDue == nil || cached.NextCheckDue.After(now) {
				return cached, nil
			}
		}
	}

	agg := domain.WatchlistResult{Lists: []string{}}
	seen := map[string]bool{}
	for _, p := range s.providers {
		hit, err := p.Screen(ctx, q)
		if err != nil {
			return domain.WatchlistResult{}, err
		}
		if !hit.OnList {
			continue
		}
		agg.OnList = true
		for _, l := range hit.Lists {
			if l != "" && !seen[l] {
				seen[l] = true
				agg.Lists = append(agg.Lists, l)
			}
		}
		if agg.Program == "" {
			agg.Program = hit.Program
		}
		if hit.Score != nil && (agg.MatchScore == nil || *hit.Score > *agg.MatchScore) {
			agg.MatchScore = hit.Score
		}
	}

	due := now.Add(s.ttl)
	agg.CheckedAt = now
	agg.NextCheckDue = &due
	if s.cache != nil && q.SubjectKey != "" {
		if err := s.cache.Put(ctx, q.SubjectKey, agg); err != nil {
			return domain.WatchlistResult{}, err
		}
	}
	return agg, nil
}

// dbCache is the pgx-backed Cache over hermenea.watchlist_cache (raw SQL — the table is operational and
// not part of the sqlc surface). Result metadata is stored as JSON.
type dbCache struct {
	conn db.DBTX
}

// NewDBCache builds the cache over a db.DBTX (the hermenea pool).
func NewDBCache(conn db.DBTX) Cache { return dbCache{conn: conn} }

// cachedResult is the JSON shape persisted in watchlist_cache.result.
type cachedResult struct {
	OnList       bool       `json:"onList"`
	Lists        []string   `json:"lists"`
	Program      string     `json:"program,omitempty"`
	MatchScore   *float64   `json:"matchScore,omitempty"`
	CheckedAt    time.Time  `json:"checkedAt"`
	NextCheckDue *time.Time `json:"nextCheckDue,omitempty"`
}

func (c dbCache) Get(ctx context.Context, subjectKey string) (domain.WatchlistResult, bool, error) {
	var raw []byte
	err := c.conn.QueryRow(ctx,
		`SELECT result FROM hermenea.watchlist_cache WHERE subject_key = $1`, subjectKey).Scan(&raw)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return domain.WatchlistResult{}, false, nil
		}
		return domain.WatchlistResult{}, false, nil // a cache miss/read error is non-fatal — screen fresh
	}
	var cr cachedResult
	if err := json.Unmarshal(raw, &cr); err != nil {
		return domain.WatchlistResult{}, false, nil
	}
	lists := cr.Lists
	if lists == nil {
		lists = []string{}
	}
	return domain.WatchlistResult{
		OnList:       cr.OnList,
		Lists:        lists,
		Program:      cr.Program,
		MatchScore:   cr.MatchScore,
		CheckedAt:    cr.CheckedAt,
		NextCheckDue: cr.NextCheckDue,
	}, true, nil
}

func (c dbCache) Put(ctx context.Context, subjectKey string, r domain.WatchlistResult) error {
	lists := r.Lists
	if lists == nil {
		lists = []string{}
	}
	payload, err := json.Marshal(cachedResult{
		OnList:       r.OnList,
		Lists:        lists,
		Program:      r.Program,
		MatchScore:   r.MatchScore,
		CheckedAt:    r.CheckedAt,
		NextCheckDue: r.NextCheckDue,
	})
	if err != nil {
		return err
	}
	expires := r.CheckedAt
	if r.NextCheckDue != nil {
		expires = *r.NextCheckDue
	}
	_, err = c.conn.Exec(ctx,
		`INSERT INTO hermenea.watchlist_cache (subject_key, result, checked_at, expires_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (subject_key) DO UPDATE
		   SET result = EXCLUDED.result, checked_at = EXCLUDED.checked_at,
		       expires_at = EXCLUDED.expires_at, updated_at = now()`,
		subjectKey, payload, r.CheckedAt, expires)
	return err
}
