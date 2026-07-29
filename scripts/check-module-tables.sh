#!/usr/bin/env bash
# check-module-tables.sh — module-table boundary lint (architecture review R-08, Option A).
#
# The hexagonal module boundary is otherwise enforced only socially: one root sqlc.yaml generates
# the whole schema into every module package, so any module could silently author a query against
# another module's tables and sqlc would happily compile it — the exact bug class the architecture
# exists to prevent. This script reads each module's queries/*.sql and fails when a query references
# a table owned by a *different* module, outside a small reviewed allowlist of legitimate
# cross-module reads (e.g. the PDP reading the tenant unit DAG).
#
# It lints the hand-authored SQL (the real violation surface), not the generated *sql/models.go
# structs (which are duplicated schema-wide by design and are noise here). See
# docs/architecture/review-2026-07.md#r-08.
#
# Usage: bash scripts/check-module-tables.sh   (run from anywhere; resolves the repo root itself)
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# --- Reviewed data (the "reviewed" half of Option A) -------------------------------------------
#
# REGISTRY: table prefix -> owning module. NOTE the module *directory* name is frequently NOT the
# table prefix (authorization owns `authz`, localization owns `i18n`, identityfederation owns
# `account`), and some modules own more than one prefix (geo owns geo+location, language owns
# language+writing). Ownership below is what makes a longest-prefix reference resolve to a module.
declare -A REGISTRY=(
  [account]=identityfederation
  [audit]=audit
  [authz]=authorization
  [company]=company
  [document]=document
  [education]=education
  [geo]=geo
  [i18n]=localization
  [language]=language
  [location]=geo
  [membership]=membership
  [order]=order
  [person]=person
  # person god-module split (D-PersonModuleSplit, review-2026-07 R-09): per-table ownership for the
  # profile / sensitive concern modules. These exact table names are LONGER than the bare `person`
  # prefix above, so the longest-prefix resolver routes them to their concern module; the three core
  # aggregate tables (person_persons / person_ranks / person_name_variants) fall through to `person`.
  [person_citizenships]=personprofile
  [person_residences]=personprofile
  [person_addresses]=personprofile
  [person_emails]=personprofile
  [person_phones]=personprofile
  [person_call_signs]=personprofile
  [person_email_types]=personprofile
  [person_phone_types]=personprofile
  [person_platforms]=personprofile
  [person_messenger_links]=personprofile
  [person_social_accounts]=personprofile
  [person_social_account_handles]=personprofile
  [person_languages]=personprofile
  [person_relation_types]=personprofile
  [person_partnerships]=personprofile
  [person_kinships]=personprofile
  [person_guardianships]=personprofile
  [person_sponsorships]=personprofile
  [person_next_of_kin]=personprofile
  [person_associations]=personprofile
  [person_government_positions]=personprofile
  [person_lobbying_relationships]=personprofile
  [person_external_references]=personprofile
  [person_physical_descriptions]=personsensitive
  [person_distinguishing_marks]=personsensitive
  [person_ethnicity_types]=personsensitive
  [person_ethnicity_type_languages]=personsensitive
  [person_ethnicity_type_countries]=personsensitive
  [person_ethnicities]=personsensitive
  [person_party_memberships]=personsensitive
  [person_watchlist_matches]=personsensitive
  [person_regulatory_sanctions]=personsensitive
  [person_crypto_wallets]=personsensitive
  [person_personality]=personsensitive
  [person_political_leaning]=personsensitive
  [person_health_records]=personsensitive
  [person_insurance]=personsensitive
  [platform]=platform     # seed-only; no domain query dir. Any real-module read of it must be reviewed.
  [rank]=rank
  [religion]=religion     # seed-only; no domain query dir.
  [tenant]=tenant
  [writing]=language
)

# ALLOW: reviewed cross-module reads, one "module:prefix" per line. Each is a deliberate, verified
# read across a boundary; adding one is a reviewed change (it widens a module's reach).
ALLOW="
authorization:tenant   # PDP over the unit DAG: tenant_graphs, tenant_unit_closure, tenant_units
company:tenant         # company org profiles hang off tenant_organizations
education:person       # enrollments / qualifications / memberships on person_* links
education:tenant       # education org profiles hang off tenant_organizations
localization:language  # locale <-> languoid reconciliation
membership:authz       # membership joins role assignments for effective authority
membership:person      # membership resolves person names/records
membership:tenant      # membership targets units in the tenant DAG
person:rank            # person core holds exactly one rank per system
# person concern-module cross-reads (D-PersonModuleSplit, review-2026-07 R-09): each concern module
# verifies the parent person exists (PersonExists on person_persons) and does its own reference lookups.
personprofile:person   # parent-existence guard on person_persons before touching a person's directory rows
personprofile:geo      # phone country lookups (geo_countries)
personprofile:language # person SPEAKS languoid links (language_languoids)
personsensitive:person # parent-existence guard + watchlist identity read on person_persons
personsensitive:geo    # ethnicity-type country associations (geo_countries)
personsensitive:language # ethnicity-type language associations (language_languoids)
rank:geo               # rank preset import resolves geo_countries by code
tenant:language        # unit language links
# Person facet block (M56 / D-ObjectFacets). The person facet vocabulary spans three cross-module
# probes — hasAccount (account_accounts), unitId (membership_memberships) and its subtree expansion
# (tenant_unit_closure / tenant_graphs) — and the SAME block is bound into person's two admin list
# queries AND membership's three visibility queries.
#
# Why the block is duplicated rather than routed through a seam: the predicates must run INSIDE the
# query, before the LIMIT (review-2026-07 R-06). Calling a membership seam from person's admin path
# would either return up to 10^5 person ids to Go, breaking filter-before-LIMIT, or give the admin
# and read-scope paths different query bodies — which is the drift the shared PersonFilter and the
# SQL narg-parity test exist to prevent. person:membership inverts the usual direction knowingly.
membership:account     # hasAccount facet: EXISTS over active accounts, inside the visibility queries
person:account         # hasAccount facet: the same probe on the instance-admin list/search queries
person:membership      # unitId facet: active-membership probe, folded in before the LIMIT
person:tenant          # unitId facet: subtree expansion over tenant_unit_closure / tenant_graphs
"

# EXEMPT: modules with no domain boundary to protect on their query surface.
#   dataimport — the ingestion fan-in (D-DataIngestion / D-Hermenea); writes into geo/language/
#     person/platform/rank/religion/writing/i18n by design. Cross-cutting write is its role.
#   hermenea   — separate binary / separate DB / own migrations; holds no oikumenea.* tables.
declare -A EXEMPT=([dataimport]=1 [hermenea]=1)

# --- Resolver ----------------------------------------------------------------------------------
# Registry prefixes, longest first, so `i18n_*`, `location_*`, `writing_*`, `language_writing_*`
# resolve to the right owner rather than a shorter accidental match.
mapfile -t PREFIXES_BY_LEN < <(printf '%s\n' "${!REGISTRY[@]}" | awk '{ print length, $0 }' | sort -rn | cut -d' ' -f2-)

# resolve_owner <ident> -> echoes "prefix owner" if the ident is a known table, else nothing
# (unknown => a shared helper/function like new_id()/set_config/ensure_audit_partition; skipped).
resolve_owner() {
  local ident="$1" p
  for p in "${PREFIXES_BY_LEN[@]}"; do
    if [[ "$ident" == "$p" || "$ident" == "$p"_* ]]; then
      printf '%s %s\n' "$p" "${REGISTRY[$p]}"
      return 0
    fi
  done
  return 1
}

allowed() { # allowed <module> <prefix>
  grep -qE "^[[:space:]]*$1:$2([[:space:]]|#|$)" <<<"$ALLOW"
}

# --- Scan --------------------------------------------------------------------------------------
violations=0
scanned=0
for dir in internal/*/adapters/queries; do
  [ -d "$dir" ] || continue
  module="${dir#internal/}"; module="${module%%/*}"
  [ -n "${EXEMPT[$module]:-}" ] && continue

  while IFS= read -r sqlfile; do
    scanned=$((scanned + 1))
    # line:oikumenea.ident, one per match
    while IFS=: read -r lineno match; do
      ident="${match#oikumenea.}"
      read -r prefix owner < <(resolve_owner "$ident") || continue   # unknown => function/shared
      [ "$owner" = "$module" ] && continue                            # own table
      allowed "$module" "$prefix" && continue                        # reviewed cross-read
      printf '  %s:%s  ->  oikumenea.%s  (owned by module "%s")\n' \
        "$sqlfile" "$lineno" "$ident" "$owner" >&2
      violations=$((violations + 1))
    done < <(grep -noE 'oikumenea\.[a-z_0-9]+' "$sqlfile" || true)
  done < <(find "$dir" -name '*.sql' -type f | sort)
done

echo "checked $scanned query file(s) across $(ls -d internal/*/adapters/queries 2>/dev/null | wc -l) module(s) (dataimport, hermenea exempt)"
if [ "$violations" -gt 0 ]; then
  echo >&2
  echo "FAIL: $violations cross-module table reference(s) outside the reviewed allowlist." >&2
  echo "If a reference is a legitimate cross-module read, add a reviewed 'module:prefix' line to" >&2
  echo "ALLOW in scripts/check-module-tables.sh; otherwise keep queries within the module's tables." >&2
  exit 1
fi
echo "module-table boundaries OK"
