// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Cross-request grant cache (review-2026-07 R-01.2 / D-AuthzGrantCache). Caches a subject's
// (isAdmin, grants) per process, validated against the single-row authz_epoch counter that every
// authority-mutating transaction bumps:
//
//   - entry fresh (validated < grantCacheTTL ago) → served with ZERO database reads;
//   - entry stale → ONE single-row epoch read; unchanged epoch revalidates the entry, a changed
//     epoch forces the full 2-query authority fetch;
//   - miss → epoch read FIRST, then the fetch (a concurrent bump makes the stored entry
//     conservatively stale, never wrongly fresh).
//
// Revocation latency: a grant/revoke/role-edit is visible everywhere within grantCacheTTL, and
// immediately in the process that performed it (the mutating write resets the local cache after
// commit). The RLS backstop underneath is exact/live (D-RLSLiveReach), so a stale ALLOW cannot
// read revoked-away rows on RLS-guarded tables. Recorded as D-AuthzGrantCache in decisions.md.
package application

import (
	"context"
	"sync"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/palantir/pkg/metrics"
	"golang.org/x/sync/singleflight"
)

// Grant-cache metrics (architecture review R-20). Emitted on the witchcraft metrics registry the
// server already exposes (metrics.FromContext(ctx) → the DefaultMetricsRegistry witchcraft emits from
// via metric.1), so no plumbing beyond the request/boot ctx. Alarm-worthy: a hit rate that falls off
// steady-state signals a TTL misconfig or an epoch-bump storm (docs/modules/platform.md).
const (
	metricGrantCacheHits          = "authz.grantcache.hits"          // served with zero DB reads
	metricGrantCacheMisses        = "authz.grantcache.misses"        // full 2-query authority fetch
	metricGrantCacheRevalidations = "authz.grantcache.revalidations" // stale-by-clock, current-by-epoch
	metricGrantCacheResets        = "authz.grantcache.resets"        // whole-map drop (mutation or overflow)
	metricGrantCacheEntries       = "authz.grantcache.entries"       // current entry count (gauge)
)

// grantCacheTTL is the trust window for a validated entry — and therefore the cross-process
// revocation-latency bound recorded in D-AuthzGrantCache.
const grantCacheTTL = 2 * time.Second

// grantCacheMaxEntries caps memory; on overflow the whole map is dropped (subjects re-fetch — a
// startup-shaped cost, simpler and safer than an eviction policy for a cache this cheap to refill).
const grantCacheMaxEntries = 10_000

type grantEntry struct {
	epoch       int64
	validatedAt time.Time
	isAdmin     bool
	grants      []domain.ActiveGrant // immutable after store — the PDP only reads
}

type grantCache struct {
	mu      sync.Mutex
	entries map[string]*grantEntry // subject person RID → entry
	sf      singleflight.Group     // per-subject stampede control on miss/revalidate
	now     func() time.Time       // injectable clock for tests
}

func newGrantCache() *grantCache {
	return &grantCache{entries: make(map[string]*grantEntry), now: time.Now}
}

func (c *grantCache) lookup(subject string) (*grantEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[subject]
	return e, ok
}

func (c *grantCache) store(ctx context.Context, subject string, e *grantEntry) {
	c.mu.Lock()
	overflow := len(c.entries) >= grantCacheMaxEntries
	if overflow {
		c.entries = make(map[string]*grantEntry)
	}
	c.entries[subject] = e
	n := len(c.entries)
	c.mu.Unlock()
	if overflow {
		metrics.FromContext(ctx).Counter(metricGrantCacheResets).Inc(1) // overflow is a reset-shaped drop
	}
	metrics.FromContext(ctx).Gauge(metricGrantCacheEntries).Update(int64(n))
}

// reset drops every entry — called after this process commits an authority-mutating write so the
// mutation is visible to the next local call immediately (cross-process visibility is bounded by
// the TTL via the epoch).
func (c *grantCache) reset(ctx context.Context) {
	c.mu.Lock()
	c.entries = make(map[string]*grantEntry)
	c.mu.Unlock()
	reg := metrics.FromContext(ctx)
	reg.Counter(metricGrantCacheResets).Inc(1)
	reg.Gauge(metricGrantCacheEntries).Update(0)
}

// cachedAuthority resolves (isAdmin, grants) through the cache, hitting the database only per the
// protocol in the package comment.
func (s *Service) cachedAuthority(ctx context.Context, subject string) (bool, []domain.ActiveGrant, error) {
	c := s.grants
	if e, ok := c.lookup(subject); ok && c.now().Sub(e.validatedAt) < grantCacheTTL {
		metrics.FromContext(ctx).Counter(metricGrantCacheHits).Inc(1)
		return e.isAdmin, e.grants, nil
	}
	v, err, _ := c.sf.Do(subject, func() (any, error) {
		// Re-check under the flight: a concurrent caller may have refreshed while we queued.
		if e, ok := c.lookup(subject); ok && c.now().Sub(e.validatedAt) < grantCacheTTL {
			metrics.FromContext(ctx).Counter(metricGrantCacheHits).Inc(1)
			return e, nil
		}
		repo := s.newRepo(s.pool)
		epoch, err := repo.ReadAuthzEpoch(ctx)
		if err != nil {
			return nil, err
		}
		if e, ok := c.lookup(subject); ok && e.epoch == epoch {
			// Stale by clock, current by epoch: revalidate without re-running the grants join.
			metrics.FromContext(ctx).Counter(metricGrantCacheRevalidations).Inc(1)
			fresh := &grantEntry{epoch: epoch, validatedAt: c.now(), isAdmin: e.isAdmin, grants: e.grants}
			c.store(ctx, subject, fresh)
			return fresh, nil
		}
		metrics.FromContext(ctx).Counter(metricGrantCacheMisses).Inc(1)
		isAdmin, grants, err := s.fetchAuthority(ctx, subject)
		if err != nil {
			return nil, err
		}
		fresh := &grantEntry{epoch: epoch, validatedAt: c.now(), isAdmin: isAdmin, grants: grants}
		c.store(ctx, subject, fresh)
		return fresh, nil
	})
	if err != nil {
		return false, nil, err
	}
	e := v.(*grantEntry)
	return e.isAdmin, e.grants, nil
}
