# Pinax — the reference plane

> **Status: designed (M45), not yet built.** Binding design lives in
> [`roadmap-decisions.md` → D-Pinax](roadmap-decisions.md); this note is the readable overview of the
> plane and its seeding contract. It becomes binding-against-code when M45 enters implementation.

**Pinax** (πίναξ — an ancient catalogue/register; Callimachus' *Pinakes* was the first library
catalogue) is the name for **go-oikumenea's reference plane**: the instance-global, read-mostly
**world-model** catalogs, grouped as a **naming convention** — *not* a new RID service, *not* a
separate schema, *not* a separate database. It is the visible boundary between the *world model* and
the *operational core* that the reference modules previously lacked.

## The three buckets

Classify every catalog-ish entity on two axes — **source of truth** (external-upstream / internal-
curated / operator-authored) and **tenant scope** (instance-global / org-scoped):

1. **Reference plane (`pinax`)** — `external|curated + global`, read-mostly, upstream/curated-versioned.
   **This document.**
2. **Operational core** — `operator-authored + org-scoped`: person, membership, order, unit,
   position. Provenance is "who asserted it / which order," not "which import."
3. **Small structural type/kind catalogs** — `curated + global` but tiny, FK'd at migration time,
   changed only with the schema (relation/phone/email types, document schemes, `*_kinds`,
   `platform_legal_basis_kinds`, …). **These stay migration-seeded** — moving them to presets buys
   nothing and reopens FK-ordering pain. They still carry the `origin` marker for uniformity.

The chaos M45 fixes was that bucket 1 was scattered among bucket-2/3 modules with **no boundary** and
**two divergent load paths** (rank seeds baked into migration `0004`; languages loaded via the import
path).

## Plane membership

| Catalog | Service | Bundled preset |
|---|---|---|
| `platform_colors` | platform (1) | colors |
| `geo_countries` | geo (12) | countries (WOF enrichment; skeleton in migration) |
| `language_languoids` | language (13) | languages (Glottolog) |
| `writing_systems` + `language_writing_systems` | language (13) | writing-systems + CLDR wiring |
| `rank_*` systems | rank | ranks (UA + US per branch) |
| `religion_taxa` | religion (16) | religions |
| `person_ethnicity_types` (+ closure / `_languages` / `_countries`) | person (6) | ethnicities (Factbook) |

Ethnicity is here because **the vocabulary is public reference data**; only the *person↔ethnicity
declaration* (`person_ethnicities`) is Art. 9 `pii:special`, stays **envelope-encrypted**, and lives in
the operational core untouched.

## The seeding contract

**One import pipeline, two connector kinds.** Bounded/curated world content ships as **bundled YAML
presets**; the massive `geo_places` gazetteer stays a **remote hermenea connector** (D-GeoPlaces).
Both funnel through the *same* application import service that the HTTP `POST /import/{objectType}`
wraps — same provenance `(source, source_version, imported_at)`, same idempotency. "Seed" is just a
`bundled_file` source.

**`go:embed` + boot autoseed, in oikumenea.** Presets are embedded in the oikumenea binary and
self-seeded on boot (in-process, not over HTTP) — so a **fresh oikumenea is usable standalone**;
hermenea is reserved for the big/live connectors. Config **`pinax.autoseed`** (default `true`) gates
it; a `pinax_seed_state` table version-gates it so a warm DB does an O(#presets) no-op check, not a
27k-row re-upsert, every restart. `oikumenea seed --reconcile` covers `autoseed:false` and manual
refresh.

**Algorithm — create-if-absent, fill-if-empty, never delete.** Matched on natural-key `code`:

- **absent → INSERT** (`origin='seeded'`); **present → do nothing**
  (`INSERT … ON CONFLICT (code) DO NOTHING`).
- Migration-seeded **skeletons** (locales, countries) are **enriched fill-if-empty**: the seeder fills
  `coordinates` / translations / `color_id` only where `NULL`/blank, and **never overwrites** a
  non-empty value.
- A code that **vanishes upstream persists** (no auto-delete / auto-deprecate ⇒ no orphaned operational
  FK — the general fix for the Crimea-FK class of bug).
- Upstream **corrections** to seeded rows propagate **only** via explicit `--reconcile` (touches
  `origin='seeded'` only).

> **Invariant:** the boot seeder never overwrites existing data — it only fills gaps.

**`origin` marker.** `origin TEXT NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'))`
on every seeded reference table. The seeder writes `seeded`; ordinary API inserts default `operator`;
reconcile touches `seeded` only. It **labels provenance**, **protects operator-created rows**, and
**gates `--reconcile`**. Operator-*edited names* don't collide because i18n is a `locale → text` map
with per-entry provenance (D-i18n): a seeded `cldr|curated` entry and an operator/official entry
coexist, and reconcile touches only the former.

**Translations.** CLDR where authoritative (country + language display names, `ukr`/`eng`); hand-
authored + `source:curated` where no source exists (religion / rank / ethnicity). `language↔writing_
system` and `language↔country` wirings are CLDR-derived in the generator, not hand-authored.

## Why not a separate service / DB

The design leans on **single-Postgres FK integrity** — closure tables, `person SPEAKS language`,
ethnicity↔country↔language, rank-on-person. A physical split trades that for distributed-consistency
problems (validate-on-write hops, version skew, orphan RIDs) and buys nothing: reference data is
read-mostly with no independent scaling/release/team force. So the plane is **logical**. But because
the `/import` boundary is preserved, the plane remains an **extraction seam** — the same
extraction-ready posture as the modular monolith — if a real force (a shared multi-deployment
gazetteer, a public dataset) ever appears. Earn the split; don't pre-pay it.

## Licensing — the presets are not Apache-2.0

The bundled presets are `go:embed`-ed, so **shipping oikumenea ships their data under their upstream
terms**, which the repository's Apache-2.0 [`LICENSE`](../../LICENSE) does not cover. Each preset
declares its own `license:` front-matter field; two are **share-alike** (`countries.yaml` is
ODbL-1.0, `religions.yaml` is CC-BY-SA-4.0) and therefore bind downstream redistributors.
Per-dataset obligations, the required attribution, and how to build without the share-alike presets:
**[data-licenses.md](../reference/data-licenses.md)**. Adding a preset means updating that document —
`TestPresetLicensesAreDocumented` fails the build otherwise.

See also: [hermenea](../modules/hermenea.md) (the ingestion companion), [conventions](conventions.md)
(schema/RID/i18n rules), [upgrade-safety](upgrade-safety.md) (non-destructive migrations),
[data-licenses](../reference/data-licenses.md) (what the bundled data obliges you to do).
