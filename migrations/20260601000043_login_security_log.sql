-- Migration 0043_login_security_log: the first-party login/IP security log (M37 / D-LoginSecurityLog).
--
-- A security telemetry stream on the identity-federation seam: the OIDC/JWKS validation middleware
-- already sees a validated request's account, client IP and user-agent, so it records a bounded,
-- de-duplicated login/IP history without the service becoming an auth provider (L-AuthzOnly holds — no
-- tokens issued, no credentials stored). NOT OSINT enrichment. pii:contact, retention-bounded (a plain
-- DELETE sweep — the stream is deduped, so no partitioning), and purge-erased with the person.
--
-- Volume: NOT a per-request firehose. The middleware de-dupes to ONE row per (account_id, context, ip)
-- per configurable window (default ~1h app-side) — a bump of last_seen_at + occurrence_count within the
-- window, else a new row. So the table stays a genuine security log, not a request log.
--
-- No RLS: like account_accounts (its parent), a login event has no unit_id and is not reach-scoped;
-- reads are gated by the app-layer PDP (account.security-log.read, instance-scope). Expand-only.
-- Depends on 0008 (account_accounts) and 0011 (the app-role default privileges cover this new table).

-- account_login_events — an account Object (9,1,4 = login_event; the next account object after
-- service_principal 9,1,3). Append-with-dedup; no soft-delete (a security log — erasure on purge is a
-- hard delete). resolved_* / is_vpn / is_tor are the IP-intelligence seam: NULL until a resolver ships
-- (deferred — a future hermenea connector), so the MVP records raw ip + user_agent.
CREATE TABLE oikumenea.account_login_events (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(9,1,4),  -- account / object / login_event
  account_id       uuid NOT NULL REFERENCES oikumenea.account_accounts(id) ON DELETE CASCADE,
  context          text NOT NULL CHECK (context IN ('login','activity','registration')),
  ip               inet NOT NULL,
  first_seen_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at     timestamptz NOT NULL DEFAULT now(),
  occurrence_count integer NOT NULL DEFAULT 1 CHECK (occurrence_count > 0),
  -- IP-intelligence seam (nullable; resolver deferred):
  resolved_country text,     -- ISO 3166-1 alpha-2 when resolved
  resolved_isp     text,
  is_vpn           boolean,
  is_tor           boolean,
  user_agent       text,

  CONSTRAINT account_login_events_rid_shape
    CHECK (oikumenea.rid_service(id)=9 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);

-- Dedup probe (find the recent (account, context, ip) row to bump) + the read keyset (account history,
-- newest first).
CREATE INDEX account_login_events_dedup_idx
  ON oikumenea.account_login_events (account_id, context, ip, last_seen_at DESC);
-- Retention sweep + the erase-by-account fan-out.
CREATE INDEX account_login_events_sweep_idx ON oikumenea.account_login_events (last_seen_at);

COMMENT ON COLUMN oikumenea.account_login_events.id               IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.account_id       IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.context          IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.ip               IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_login_events.first_seen_at    IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.last_seen_at     IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.occurrence_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.resolved_country IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_login_events.resolved_isp     IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_login_events.is_vpn           IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.is_tor           IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.user_agent       IS 'pii:contact';

-- delete_login_events_before: the retention sweep helper (D-LoginSecurityLog; mirrors the M50
-- detach_audit_partitions_before operator seam). Returns the number of rows deleted. Retention is an
-- operator policy (login-security.retention-days; 0 = retain forever) enforced by the app calling this.
CREATE FUNCTION oikumenea.delete_login_events_before(cutoff timestamptz) RETURNS bigint
LANGUAGE sql AS $$
  WITH del AS (
    DELETE FROM oikumenea.account_login_events WHERE last_seen_at < cutoff RETURNING 1
  )
  SELECT count(*) FROM del;
$$;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0043_login_security_log', applied_at = now() WHERE singleton;
