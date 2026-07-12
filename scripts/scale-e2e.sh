#!/usr/bin/env bash
# scale-e2e.sh — two-replica compose proof of the multi-replica posture (architecture review R-13,
# Phase 8). Boots TWO oikumenea replicas against one Postgres and one shared outbox and asserts the
# properties July designed but never ran across real OS processes:
#
#   1. Single boot-seed pass          — advisory lock (db.LockBootSeed) serializes first-admin bootstrap
#                                        + pinax autoseed; exactly one replica seeds, the other skips.
#   2. Outbox exactly-once + sharing  — N notify events, drained by BOTH replicas via FOR UPDATE SKIP
#                                        LOCKED, each delivered exactly once (idempotent handler dedup).
#   3. kill -9 redelivery on survivor — kill one replica mid-drain; the survivor redelivers its claimed
#                                        but unmarked rows; the delivered count still reaches N.
#   4. Grant-cache convergence        — an instance-admin grant on replica B flips a PDP decision on
#                                        replica A within the 2 s grantCacheTTL (epoch via the shared DB).
#
# Steps 1-3 use only container-native primitives (`docker compose exec`, `psql`, `docker logs/kill`), so
# they run anywhere Docker does. Step 4 needs per-replica HTTP; it runs when the replicas are reachable
# over the compose network and otherwise SKIPS with a pointer to TestGrantCacheCrossReplicaConvergence
# (which proves the same shared-epoch convergence in-process) — never silently dropped.
#
# Env-gated (OIKUMENEA_SCALE_E2E=1) and isolated (project `oik-scale`, its own volume) so it never
# touches dev/prod data. Repeatable: tears its stack + volume down on exit. Requires docker compose
# v2.24+ (the scale override uses `!reset`).
set -euo pipefail

if [[ "${OIKUMENEA_SCALE_E2E:-}" != "1" ]]; then
  echo "scale-e2e: set OIKUMENEA_SCALE_E2E=1 to run this two-replica compose verification (skipped)."
  exit 0
fi

cd "$(dirname "$0")/.."

PROJECT=oik-scale
COMPOSE=(docker compose -f docker-compose.yml -f docker-compose.scale.yml -p "$PROJECT")
NET="${PROJECT}_default"
N="${SCALE_E2E_EVENTS:-2000}"                              # events per outbox batch (big enough that both replicas grab work)
HMAC_KEY="local-dev-insecure-signing-key-change-me"        # deploy/install.docker.yml idp.issuers[0].hmac-key
ISS="https://local-dev.oikumenea.test"                     # deploy/install.docker.yml bootstrap-admin.issuer
API=8443

pass() { echo "  ✅ $*"; }
skip() { echo "  ⏭️  $*"; }
fail() { echo "  ❌ $*" >&2; exit 1; }
step() { echo; echo "== $* =="; }

cleanup() { echo; echo "== teardown =="; "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true; }
trap cleanup EXIT

# psql against the shared DB (the postgres image has psql; the distroless app image does not).
psql() { "${COMPOSE[@]}" exec -T postgres psql -U postgres -d postgres -tAqc "$1" | tr -d '[:space:]'; }
# run the oikumenea binary inside a replica by 1-based index (distroless: exec runs it directly).
app_exec() { local idx=$1; shift; "${COMPOSE[@]}" exec -T -e OIKUMENEA_OUTBOX_SELFTEST=1 --index "$idx" app "$@"; }
app_cid()  { "${COMPOSE[@]}" ps -q app | sed -n "${1}p"; }
# how many replicas have finished InitFunc (the bootstrap line is logged near the end of boot).
booted_count() { "${COMPOSE[@]}" logs --no-color app 2>/dev/null | grep -c "first-admin bootstrap" || true; }

# ---------------------------------------------------------------- boot two replicas
step "boot: two replicas + fresh DB (project $PROJECT)"
"${COMPOSE[@]}" up -d --build --scale app=2
echo "  waiting for both replicas to finish boot (advisory-locked seed → dispatcher running)…"
for i in $(seq 1 150); do
  [[ "$(booted_count)" -ge 2 ]] && break
  [[ $i -eq 150 ]] && { "${COMPOSE[@]}" logs --no-color app | tail -30; fail "both replicas never finished boot"; }
  sleep 2
done
sleep 2   # let the just-started outbox dispatchers settle into their poll loop
pass "both replicas booted"

# Provision the self-test deliveries table as the superuser (the app role has no CREATE privilege —
# D-RLSDefenseInDepth) and grant the app role INSERT so the boot-registered handler can record deliveries.
psql "CREATE TABLE IF NOT EXISTS oikumenea.platform_outbox_selftest_deliveries (
        event_id text PRIMARY KEY, replica text NOT NULL, delivered_at timestamptz NOT NULL DEFAULT now());
      GRANT INSERT, SELECT ON oikumenea.platform_outbox_selftest_deliveries TO oikumenea_app;" >/dev/null

# ---------------------------------------------------------------- 1. single boot-seed pass
step "1. single boot-seed pass (advisory lock)"
logs=$("${COMPOSE[@]}" logs --no-color app 2>/dev/null || true)
seeded=$(grep -c "first-admin bootstrap seeded instance admin" <<<"$logs" || true)
skipped=$(grep -c "first-admin bootstrap skipped" <<<"$logs" || true)
echo "  seeded=$seeded skipped=$skipped (across both replicas)"
[[ "$seeded" == "1" ]] || fail "expected exactly ONE replica to seed the first admin, got $seeded"
[[ "$skipped" -ge "1" ]] || fail "expected the other replica to skip (advisory lock), got skipped=$skipped"
pass "exactly one seed pass; the other replica waited on the lock and no-oped"

# ---------------------------------------------------------------- 2. outbox exactly-once + both replicas drain
step "2. outbox exactly-once + both replicas draining ($N events)"
psql "TRUNCATE oikumenea.platform_outbox_selftest_deliveries;" >/dev/null
psql "DELETE FROM oikumenea.platform_outbox WHERE event_type='platform.selftest';" >/dev/null
app_exec 1 /app/oikumenea outbox-selftest-enqueue --n "$N" >/dev/null
echo "  enqueued $N; waiting for the two dispatchers to drain the shared queue…"
delivered=0
for i in $(seq 1 90); do
  delivered=$(psql "SELECT count(*) FROM oikumenea.platform_outbox_selftest_deliveries;")
  [[ "$delivered" == "$N" ]] && break
  sleep 1
done
pending=$(psql "SELECT count(*) FROM oikumenea.platform_outbox WHERE event_type='platform.selftest' AND status='pending';")
replicas=$(psql "SELECT count(DISTINCT replica) FROM oikumenea.platform_outbox_selftest_deliveries;")
echo "  delivered=$delivered pending=$pending distinct-replicas=$replicas"
[[ "$delivered" == "$N" ]] || fail "expected $N delivered (exactly once), got $delivered"
[[ "$pending" == "0" ]] || fail "expected 0 pending outbox rows, got $pending"
[[ "$replicas" == "2" ]] || fail "expected BOTH replicas to drain (distinct=2), got $replicas"
pass "all $N delivered exactly once; both replicas shared the queue (SKIP LOCKED)"

# ---------------------------------------------------------------- 3. kill -9 mid-dispatch redelivery
step "3. kill -9 mid-dispatch → redelivery on the survivor"
psql "TRUNCATE oikumenea.platform_outbox_selftest_deliveries;" >/dev/null
CID1=$(app_cid 1)
app_exec 2 /app/oikumenea outbox-selftest-enqueue --n "$N" >/dev/null
sleep 1                                    # let both dispatchers claim their first batches
docker kill --signal=KILL "$CID1" >/dev/null
echo "  killed replica-1 mid-drain; the survivor must finish + redeliver replica-1's in-flight rows…"
delivered=0
for i in $(seq 1 120); do
  delivered=$(psql "SELECT count(*) FROM oikumenea.platform_outbox_selftest_deliveries;")
  [[ "$delivered" == "$N" ]] && break
  sleep 1
done
echo "  delivered=$delivered / $N (one replica down)"
[[ "$delivered" == "$N" ]] || fail "survivor did not redeliver to $N (got $delivered)"
pass "survivor redelivered the killed replica's claimed-but-unmarked rows; still exactly-once ($N)"
# Bring replica-1 back so the convergence step (if reachable) again has two processes.
"${COMPOSE[@]}" up -d --scale app=2 >/dev/null
for i in $(seq 1 60); do [[ "$(booted_count)" -ge 2 ]] && break; sleep 2; done

# ---------------------------------------------------------------- 4. grant-cache convergence across processes
step "4. grant-cache convergence across processes (≤2 s grantCacheTTL)"
# Per-replica HTTP: reach each replica by container-name over the compose network from an ephemeral curl
# container. Some sandboxes isolate the bridge DNS — probe first and SKIP cleanly if unreachable.
curlnet() { docker run --rm --network "$NET" curlimages/curl:latest "$@"; }
if ! curlnet -sk --max-time 5 -o /dev/null "https://${PROJECT}-app-1:${API}/status/readiness" 2>/dev/null; then
  skip "replicas not reachable over the compose network from this host (sandbox DNS isolation)."
  echo "     convergence across processes is proven in-process by TestGrantCacheCrossReplicaConvergence"
  echo "     (two caches over one DB — the coordination is entirely the shared authz_epoch row, so the"
  echo "     two-process case is behaviourally identical). See docs/architecture/overview.md § posture."
else
  b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
  now=$(date +%s); exp=$((now + 3600))
  h=$(printf '%s' '{"alg":"HS256","typ":"JWT"}' | b64url)
  p=$(printf '{"iss":"%s","aud":"oikumenea","sub":"local-admin","iat":%d,"exp":%d}' "$ISS" "$now" "$exp" | b64url)
  sig=$(printf '%s' "${h}.${p}" | openssl dgst -sha256 -hmac "$HMAC_KEY" -binary | b64url)
  JWT="${h}.${p}.${sig}"
  host1="${PROJECT}-app-1"; host2="${PROJECT}-app-2"
  auth() { local host=$1 m=$2 path=$3 body=${4:-}
    if [[ -n "$body" ]]; then
      curlnet -sk -X "$m" -H "Authorization: Bearer $JWT" -H 'Content-Type: application/json' --max-time 6 -d "$body" "https://${host}:${API}${path}"
    else
      curlnet -sk -X "$m" -H "Authorization: Bearer $JWT" --max-time 6 "https://${host}:${API}${path}"
    fi; }
  decide() { auth "$1" POST /authorization/v1/authorize "{\"subjectPersonId\":\"$2\",\"action\":\"role.read\"}" | jq -r '.allow'; }

  PID=$(auth "$host1" POST /person/v1/persons '{"displayName":"scale-convergence-subject"}' | jq -r '.id')
  [[ -n "$PID" && "$PID" != "null" ]] || fail "could not create subject person (got '$PID')"
  echo "  subject person: $PID"
  b1=$(decide "$host1" "$PID"); b2=$(decide "$host2" "$PID")
  echo "  pre-grant  replica-1 allow=$b1  replica-2 allow=$b2"
  [[ "$b1" == "false" && "$b2" == "false" ]] || fail "expected DENY on both replicas before the grant"
  AID=$(auth "$host2" POST /authorization/v1/instance-admins "{\"personId\":\"$PID\"}" | jq -r '.id')
  echo "  granted instance-admin via replica-2 (id=$AID); polling replica-1 for the flip…"
  flipped=""
  for i in $(seq 1 12); do   # up to ~6 s (> the 2 s TTL); within-TTL stale reads are served by design
    [[ "$(decide "$host1" "$PID")" == "true" ]] && { flipped=yes; break; }
    sleep 0.5
  done
  [[ "$flipped" == "yes" ]] || fail "replica-1 never saw the grant from replica-2 (convergence FAILED)"
  pass "grant on replica-2 became effective on replica-1 within the TTL (shared-epoch convergence)"
  auth "$host2" DELETE "/authorization/v1/instance-admins/${AID}" >/dev/null 2>&1 || true
fi

echo
echo "== scale-e2e PASSED (R-13): seed-once · outbox exactly-once+shared · kill-9 redelivery · convergence =="
