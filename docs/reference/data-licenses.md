# Data licenses — the bundled reference plane

> Reads: [`../../LICENSE`](../../LICENSE) (Apache-2.0, the **software**) ·
> [`../../NOTICE`](../../NOTICE) (required attribution) ·
> [`../architecture/pinax.md`](../architecture/pinax.md) (what the reference plane is).

**The repository has two licensing surfaces, and one license file does not cover both.**

| Surface | What it is | License |
|---|---|---|
| **Software** | `internal/`, `pkg/`, `cmd/`, `tools/`, `api/`, `migrations/`, `clients/`, `web/` | **Apache-2.0** ([LICENSE](../../LICENSE)) |
| **Bundled reference data** | `internal/pinax/presets/*.yaml` — `go:embed`-ed into the binary (`internal/pinax/preset.go`) | **Per dataset** — see below |

This split is not bookkeeping. Two of the bundled datasets are **share-alike**, so a
redistributor's obligations do not stop at the Apache license. Every preset already declares its
own terms in a `license:` front-matter field; this document is the human-readable expansion of
those fields, and `TestPresetLicensesAreDocumented` fails the build if the two drift apart.

## Per-dataset terms

| Preset | Upstream source | License | Redistributor must |
|---|---|---|---|
| `languages.yaml` | [Glottolog](https://glottolog.org) 5.3 | `CC-BY-4.0` | **Attribute** (NOTICE carries the citation) |
| `translations.yaml` | [Unicode CLDR](https://cldr.unicode.org) + curated | `CC-BY-4.0+curated` (CLDR strings under `Unicode-3.0`; the curated additions released under CC-BY-4.0 to match) | **Attribute** both |
| `writing-systems.yaml` | Unicode CLDR | `Unicode-3.0` | **Attribute**; retain the Unicode notice |
| `countries.yaml` | [mledoze/countries](https://github.com/mledoze/countries) + Natural Earth 110m | **`ODbL-1.0`** + public domain | **Attribute** *and* **share alike** — see below |
| `religions.yaml` | Curated (Wikidata CC0 + published taxonomies) | **`CC-BY-SA-4.0`** | **Attribute** *and* **share alike** — see below |
| `ethnicities.yaml` | [CIA World Factbook](https://www.cia.gov/the-world-factbook/) | `Public-Domain` | nothing (US Government work) |
| `colors.yaml` | Curated | `CC0-1.0` | nothing |
| `ranks.yaml` | Curated | `CC0-1.0` | nothing |

## The two share-alike obligations, stated plainly

**`countries.yaml` — ODbL-1.0.** The Open Database License is copyleft *for the database*. If you
publicly distribute a database **derived from** it — which includes a `geo_countries` table
populated by the pinax autoseeder and then exported, published, or shipped inside a downstream
product's dump — that derived database must be offered under ODbL. A *Produced Work* (a map, a
report, a screen rendered from the data) does **not** have to be ODbL, but must credit the source.
Running the software and querying your own deployment is not distribution and triggers nothing.

**`religions.yaml` — CC-BY-SA-4.0.** Adaptations of the religion taxonomy must be distributed under
CC-BY-SA-4.0. Extending the seeded taxa in your own deployment is fine; **publishing** the extended
taxonomy carries the license forward.

Neither obligation reaches the **code**. CC-BY-SA and ODbL are not software licenses, the presets
are data read at runtime rather than compiled logic, and embedding a file does not make the program
an adaptation of the data. The obligations attach to the *data and databases derived from it*, which
is why they sit here and not in `LICENSE`.

## If you want a build with no share-alike data

The autoseeder is optional and per-preset. Setting `pinax.autoseed: false` (or
`OIKUMENEA_PINAX_AUTOSEED=false`) stops the seeding, but the YAML is still **embedded in the
binary** — so for a genuinely clean redistribution, drop `countries.yaml` and `religions.yaml` from
`internal/pinax/presets/` and rebuild. The country **skeleton** (code + name) is migration-seeded,
not preset-seeded, so the system still boots with a usable `geo_countries`; only the ODbL
enrichment (`iso_a3`, `numeric_code`, border geometry) and the curated religion taxonomy are lost.
Both can then be re-supplied as an operator **data pack** (D-DataPacks, M54), which keeps the
licensing question with the operator who chose the data.

## Data that is *not* bundled

Fetched at runtime by [hermenea](../modules/hermenea.md) connectors, never compiled in, and
therefore the **operator's** licensing decision — not this repository's:

| Connector | Source | Terms to check |
|---|---|---|
| `wof` (geo-places gazetteer) | [Who's On First](https://whosonfirst.org) | CC-BY / CC0 per record |
| `wikidata-orgs` | [Wikidata](https://www.wikidata.org) SPARQL | CC0-1.0 |
| `interpol` (watchlists) | INTERPOL Red Notices | INTERPOL terms of use — **not** an open license |
| `regulatory-sanctions` | OFAC / EU / UN lists | per-issuer terms |

The INTERPOL and sanctions feeds in particular carry usage restrictions that no license in this
repository grants. [D-Watchlists](../architecture/roadmap-decisions.md) keeps matches to
**metadata only** partly for this reason.

## Adding a preset

1. Set the `license:` front-matter field to a valid SPDX identifier (or `Public-Domain`).
2. Add its row to the *Per-dataset terms* table above.
3. If it requires attribution, add the citation to [`NOTICE`](../../NOTICE).
4. If it is share-alike, say so plainly in *The two share-alike obligations* — a redistributor
   should not have to infer an obligation from an SPDX string.

Step 2 is enforced: the drift guard reads every preset's `license:` field and fails if it is absent
from this document.
