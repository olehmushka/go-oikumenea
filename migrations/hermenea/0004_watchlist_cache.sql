-- 0004 watchlist cache — the ≤24h short-TTL cache backing the live watchlist screening check
-- (D-Watchlists, M34). hermenea owns egress to the OFAC/EU/UN/INTERPOL providers; a screening result is
-- cached here keyed by an opaque subject key (the person RID), so a re-check within the TTL is served
-- from cache instead of re-hitting the upstreams. Only match METADATA is stored — never the lists.
-- Operational (upsert-in-place), not soft-deleted; a lapsed row is simply overwritten on the next check.
CREATE TABLE hermenea.watchlist_cache (
  subject_key text PRIMARY KEY,                 -- opaque caller key (the person RID); never sent upstream
  result      jsonb NOT NULL,                   -- the aggregated WatchlistResult metadata
  checked_at  timestamptz NOT NULL,
  expires_at  timestamptz NOT NULL,             -- checked_at + TTL (≤24h)
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Sweep support: find lapsed rows cheaply (a future eviction job; not load-bearing for correctness).
CREATE INDEX watchlist_cache_expires_idx ON hermenea.watchlist_cache (expires_at);
