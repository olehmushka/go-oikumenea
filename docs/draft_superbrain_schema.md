# Draft — "Superbrain" people-data schema (discussion material)

> **Status: discussion draft, NOT a milestone.** This document filters an external OSINT
> "people-intelligence dossier" pre-draft against what go-oikumenea already has, and records a
> per-field verdict for later milestone scoping. Nothing here is decided/binding — it does not
> touch `decisions.md`, `roadmap-decisions.md`, or the `milestones.md` stage board. Entity/field
> names are **illustrative**; no RIDs, migrations, or Conjure contracts are implied yet.

## Purpose & framing

The source pre-draft was written as a generic OSINT collection schema (8 macro-categories of
collectable fields with sensitivity tiers, sources, inference/confidence). go-oikumenea is the
opposite posture: an **authoritative, operator-owned personnel & authorization directory**. This
draft reconciles the two under three locked decisions from the review session:

- **Hybrid, marked per-field.** Every field is sorted into one verdict (legend below). Some become
  authoritative, operator-asserted directory attributes; others become a provenance-tagged
  *overlay*; some we already have; some are excluded or deferred.
- **Keep but gate.** Sensitive / special-category fields are kept, but behind the existing 5-tier
  `pii:*` classification + audit + an explicit `legal_basis`, leaning on the machinery already in
  the repo: **D-PIITiers**, **D-CryptoProvider** (envelope encryption + blind index), **D-SpecialPII**
  (M24, special-category envelope), and the `source`/`confidence` attribution pattern from
  **D-PersonSocialChannels**.
- **Live-lookup, never static** for watchlists (sanctions / PEP).

### Verdict legend

| Tag | Meaning |
|-----|---------|
| `ALREADY HAVE` | Modeled today; no new work, maybe a small gap-filler. |
| `DEVELOP` | New **authoritative** directory attribute (operator-asserted, first-party). |
| `OVERLAY` | New **provenance-tagged** store (carries `source`+`confidence`, declared-vs-inferred). |
| `LIVE-LOOKUP` | Not stored; query-time lookup with short-TTL cache + match metadata only. |
| `DEFERRED` | In scope and wanted, but design pushed to a dedicated later session. |
| `EXCLUDED` | Considered and deliberately left out; revisit only under stated conditions. |

## What we already have (the filtering anchor)

- **Names & aliases:** `person_persons` CLDR parts (title/given/given2 (patronymic)/surname/
  surname_prefix/surname2/generation/credentials/preferred) + `person_name_variants` (per-locale
  transliterations) + `person_call_signs` (позивний).
- **Documents & codes:** `document_documents` (typed catalog incl. `passport`, `national-id`,
  `driver-license`, `military-id`, `tax-id`; number/issuer/issuing-country/attr_schema) +
  envelope-encrypted `document_personal_codes` (SSN/tax/PESEL… with blind index).
- **Geo:** `person_citizenships`, `person_residences` (country + free-text region, effective-dated),
  `geo_countries`, `geo_places` (WOF gazetteer), `location_locations` (PostGIS point, app-derived
  MGRS, multi-format coordinate input).
- **Contacts & graph:** `person_emails` / `person_phones` (typed, normalized) / messenger links /
  `person_social_accounts` (with `source`+`confidence` attribution) + person↔person relationships
  (partnership, kinship, guardianship, sponsorship, **next-of-kin**, association) via
  `person_relation_types`.
- **Org & career:** `membership_memberships` + `membership_positions`, orders (наказ) with per-item
  effects, the single rank scheme (category→type→rank, NATO STANAG-2116 grades).
- **Cross-cutting:** 5-tier `pii:*` (`none`/`basic`/`contact`/`sensitive`/`special`) via
  `COMMENT ON COLUMN`; envelope encryption + blind index + crypto-erase; per-import provenance
  `(source, source_version, imported_at)` (hermenea); append-only audit log; the PDP; the language
  module (Glottolog) + writing systems.

---

## Macro-category 1 — Physical identity

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 1.1 | Legal name | `ALREADY HAVE` | basic | CLDR parts + transliterations. |
| 1.1 | Aliases / AKA / former-name | `DEVELOP` (gap) | basic | Fold into `person_name_variants`. |
| 1.2 | Driver's licence | `ALREADY HAVE` | sensitive | `driver-license` document type. |
| 1.3 | Physical description | `DEVELOP` | basic | New structured entity. |
| 1.4 | Ethnicity | `DEVELOP` | special | Declared-only, gated. |
| 1.5 | Biometric data | `EXCLUDED` | — | Highest-risk; defer. |
| 1.6 | Blood type | `DEVELOP` | sensitive | On the physical-description entity. |

- **1.1 Aliases.** We have transliterations and a `preferred` nickname and call signs, but no
  general "also-known-as / former legal name / maiden name / pseudonym / cover name." Close the gap
  by **folding aliases into `person_name_variants`** with a `variant_kind` discriminator
  (`transliteration` | `aka` | `former_legal` | `maiden` | `pseudonym` | `cover`). Alias rows may
  carry optional `source`+`confidence`.
- **1.3 Physical description.** New `person_physical_descriptions` (typed columns: `height_cm`,
  `weight_kg`, `eye_color`, `hair_color`, `build`, plus **`blood_type`** from 1.6) + child
  `person_distinguishing_marks` (`kind` ∈ tattoo|scar|piercing|birthmark, `body_location`,
  `description`). Effective-dated. `pii:basic`, **except** distinguishing marks → `pii:special`
  ceiling (a tattoo/mark can reveal Art.9 data such as religion or affiliation).
- **1.4 Ethnicity.** Self-declared **only**; catalog-typed (open catalog + i18n name, like relation
  types). `pii:special` + envelope + explicit `legal_basis` + audit. **No inferred storage.**
- **1.5 Biometric data.** Excluded for now. If ever revisited (hard requirement + legal review):
  reference/token-only — no raw, no embeddings — with mandatory access logging.

## Macro-category 2 — Location & presence

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 2.1/2.2/2.3 | Home / work / mailing address | `DEVELOP` | contact | Typed person↔location link. |
| 2.4 | GPS location history | `EXCLUDED` | — | Mass-movement telemetry; out of fit. |
| 2.5 | IP address history | `DEVELOP` | contact | First-party login security log. |

- **2.1/2.2/2.3 Addresses.** New `person_addresses` link to `location_locations`: `role`
  (`home`|`work`|`mailing`|`other`), `valid_from`/`valid_to`, `is_primary`, `source`+`confidence`.
  Upgrades country-level `person_residences` to real structured, effective-dated address history and
  reuses the PostGIS/MGRS already in `location_locations`. Work address may also be **derived** from
  the person's unit location. (A distinct mailing address ≠ home is itself a privacy-seeking signal
  worth an optional flag.)
- **2.4 GPS history.** Excluded — movement traces are out of fit for an authoritative directory.
  Revisit only with a hard lawful basis; if ever, a heavily-gated overlay (`pii:special` + envelope
  + `legal_basis` + audit, spatial index).
- **2.5 IP history.** Modeled as a **first-party login security log** on the identity-federation
  accounts seam — `account_login_events` (ip, timestamp, context login|activity|registration,
  resolved_isp, resolved_country, is_vpn, is_tor) — *not* OSINT breach enrichment.

## Macro-category 3 — Network & relationships

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 3.1 | Emergency contacts | `ALREADY HAVE` | basic | Add `emergency` relation type. |
| — | Unresolved/external people | `DEVELOP` (cross-cutting) | basic | Provisional person stubs. |

- **3.1 Emergency contacts.** Covered by `next_of_kin` + the relationship graph. Add an `emergency`
  entry to `person_relation_types` (or reuse `next_of_kin` priority) — no new entity.
- **Provisional persons (decided here, generalized below).** Relax the in-directory-only rule:
  unresolved external people exist as **provisional person records** (`status=provisional`, minimal
  PII) so every relationship edge points to a node, promoted/merged on resolution with `confidence`.

## Macro-category 4 — Financial & assets

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 4.1 | Crypto wallets | `OVERLAY` | sensitive | Attribution edge w/ source+confidence. |
| — | Compensation / payroll | (separate module) | sensitive | Operational HR, not dossier scope. |

- **4.1 Crypto wallets.** `person_crypto_wallets` overlay: `address`, `chain`, `attribution_method`,
  `source`+`confidence`, `first_seen`/`last_seen`, `balance_usd_approx`. Gated + audited. Synergy
  with sanctions screening (6.5).
- **Compensation/payroll** is noted as a deliberately-separate future module (the org as payer), not
  designed here.

## Macro-category 5 — Behavioral & psychological

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 5.1 | Personality type | `DEVELOP` | sensitive | Declared / HR-assessment only. |
| 5.2 | Political leaning (declared) | → 7.2 | special | Modeled once in party membership. |
| 5.2 | Political leaning (inferred) | `OVERLAY` | special | Separate store; never merged. |

- **5.1 Personality.** Self-reported survey or formal HR assessment only (MBTI/Big Five/DISC/
  Enneagram as a small typed sub-entity); `pii:sensitive` + `source`. **No NLP-from-text inference.**
- **5.2 Political leaning.** Declared/formal affiliation lives once in **7.2**. The inferred spectrum
  (-1..1) is a **separate** gated overlay carrying `inference_sources`+`confidence`, **never merged**
  with declared. Both `pii:special`.

## Macro-category 6 — Legal & regulatory exposure

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 6.1 | Criminal record | `DEFERRED` | sensitive | Important; design later session. |
| 6.2 | Arrest history | `DEFERRED` | sensitive | Disposition mandatory. |
| 6.3 | Court judgments | `DEFERRED` | sensitive | Civil/family/bankruptcy. |
| 6.4 | Regulatory sanctions | `OVERLAY` | sensitive | Distinct, API-ingestible. |
| 6.5 | OFAC/EU/UN/INTERPOL + PEP | `LIVE-LOOKUP` | sensitive | Never stored statically. |

- **6.1–6.3** are **in scope and important**, but their design is **deferred to a dedicated
  session**. Hard requirements to carry into it: mandatory `disposition` (arrest ≠ guilt),
  expungement/sealing **suppression**, jurisdiction-specific storage/display rules (Ban-the-Box,
  FCRA), `pii:sensitive` + gated + audited. Distinct from internal `discipline-incentive` orders.
- **6.4 Regulatory sanctions.** `person_regulatory_sanctions` (regulator, action_type, amount,
  status, date, source_url, `source`+`confidence`). Tied to a licensed professional role; many
  regulators expose structured APIs → good **hermenea** ingestion target.
- **6.5 Sanctions/PEP.** Never store the lists. Query-time lookup + short-TTL cache (≤24h); persist
  only per-person match metadata (`on_list`, `lists[]`, `program`, `last_checked`,
  `next_check_due`). PEP is derived from 7.3 government positions.

## Macro-category 7 — Political & institutional ties

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 7.1 | Wikipedia / references | `OVERLAY` | standard/basic | Generic external-references store. |
| 7.2 | Party membership | `OVERLAY` | special | Declared political affiliation. |
| 7.3 | Government positions | `OVERLAY` | basic | `pep_trigger`; feeds 6.5. |
| 7.4 | Military service (foreign) | `DEVELOP` (reuse) | sensitive | Membership vs provisional units. |
| 7.5 | Lobbying relationships | `OVERLAY` | basic | Revolving-door cross-ref w/ 7.3. |

Theme: **person↔organization affiliation edges where the org is often external** (party, gov body,
foreign military, lobbying client) → ties into provisional org entities + the companies module
(M21) + the unit graph. Modeled as **separate entities per type**:

- `person_party_memberships` (party org, role, dates) — `pii:special` (Art.9).
- `person_government_positions` (title, body, country, level, role_type, dates, `pep_trigger`
  auto-true, persists post-office) — `pii:basic`.
- `person_lobbying_relationships` (registrant, client, legislative_body, issues[], fees, filing_id,
  source_url) — `pii:basic`.
- **7.4 Military service (foreign/historical)** → **reuse membership** against **provisional org/unit
  stubs** + rank, with extra link attributes `units[]`, `deployments[]`, `discharge_type`,
  `clearance_level` (the latter two `pii:sensitive` — character & counterintelligence signals).
- **7.1** → `person_external_references` (`kind` wikipedia|news|registry|…, `url`, `external_id`,
  `categories[]`, `last_checked`, flags e.g. `disputed`). Mirrors `social_accounts` for reference
  sites; good hermenea target.

All overlay edges carry `source`+`confidence`; the org side may be an internal unit, an M21 company,
or a provisional stub.

## Macro-category 8 — Health & vulnerability

| # | Field | Verdict | Tier | Notes |
|---|-------|---------|------|-------|
| 8.1 | Hospitalization | `OVERLAY` | special | Category-level only; public-record. |
| 8.2 | Insurance provider | `OVERLAY` | sensitive | Reveals employer + health category. |
| 8.3 | Mental health | `OVERLAY` | special | Strictest gate; never inferred. |
| 8.4 | Disabilities | `OVERLAY` | special | Public-record only. |

- **8.1/8.3/8.4** → unified `person_health_records` typed (`hospitalization`|`mental_health`|
  `disability`), **category-level only** (no diagnosis), `is_public_record`, `source`. Strictest
  gate: `pii:special` + envelope (D-SpecialPII) + app-layer **need-to-know** + **full audit**.
  **Never inferred.** Reuses the M24 special-PII machinery.
- **8.2 Insurance** → `person_insurance` (type health|life|disability|ltc, provider,
  employer_sponsored, dates), `pii:sensitive`, gated.

---

## Cross-cutting principles (to formalize when these become milestones)

1. **Provisional persons + entity resolution.** `status=provisional` person stubs let edges point
   to unresolved/external entities; promotion/merge carries `confidence`. This generalizes the
   pre-draft's "every contact is a graph edge" note across emergency contacts, lobbying clients,
   military units, and wallet attribution. The unit graph (DAG) + M21 companies are the analogous
   node-space for org/institution edges.
2. **Declared vs inferred + source/confidence.** Reuse the D-PersonSocialChannels pattern
   (`source` ∈ self_declared|operator_verified|imported; `confidence` ∈ confirmed|probable|possible)
   on every overlay/attribution field. **Never merge inferred into declared.**
3. **Sensitivity gating.** Map every field to the 5-tier `pii:*`; special-category → envelope
   (D-SpecialPII) + explicit `legal_basis` + mandatory audit (extend audit action coverage).
4. **Live-lookup, never static** for sanctions/PEP/watchlists.

## Open threads for milestone sessions

- **6.1–6.3 legal records** — deferred but important; needs its own session (disposition,
  expungement suppression, jurisdiction rules).
- **Biometrics (1.5) and GPS history (2.4)** — excluded; revisit only with a hard lawful
  requirement + legal review, and (for biometrics) token/reference-only.
- **Org-entity space** — reconcile party / government body / lobbying client / foreign military
  against companies (M21), the unit graph, and provisional stubs before designing 7.x.
- **Compensation/payroll** — separate operational-HR module, intentionally out of this draft.
- **Audit coverage & `legal_basis`** — adding special-category overlays implies new audited actions
  and a standard `legal_basis` field convention; scope when the first overlay milestone lands.
