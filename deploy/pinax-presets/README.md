# pinax preset generator (D-Pinax, M45)

Reproducible generator for the **bundled reference-plane presets** that oikumenea `go:embed`s from
[`internal/pinax/presets/`](../../internal/pinax/presets/) and self-seeds on boot (create-if-absent /
fill-if-empty / never-delete, version-gated via `pinax_seed_state`). See the
[pinax plane note](../../docs/architecture/pinax.md) and D-Pinax in
[roadmap-decisions.md](../../docs/architecture/roadmap-decisions.md).

## Run

From the repo root:

```bash
go run ./deploy/pinax-presets/gen                    # regenerate all (Factbook + countries need network)
go run ./deploy/pinax-presets/gen -skip-ethnicities  # offline: everything but the live Factbook fetch
PINAX_CACHE=/tmp/pinax go run ./deploy/pinax-presets/gen   # cache downloads for offline reruns
```

Network fetches are cached under `$PINAX_CACHE` (default: the OS temp dir), so reruns are offline and
deterministic. A fetch that is unavailable falls back gracefully (countries → the committed
`deploy/geo-presets/iso-3166.json` subset without geometry; `-skip-ethnicities` leaves `ethnicities.yaml`
untouched).

## What it generates

| preset (`objectType`) | source | records | notes |
|---|---|---|---|
| `languages` (`language-scheme`) | [`../language-presets/glottolog-5.3.json`](../language-presets) | ~27k | Glottolog 5.3 forest, **topo-sorted parent-first** (the `parent_id` FK is RESTRICT); the handler rebuilds closure + `family_code`. |
| `writing-systems` (`language-scripts`) | [`../language-presets/cldr-scripts.json`](../language-presets) | ~1k | CLDR language↔ISO-15924 links; `dependsOn: [languages]`. |
| `countries` (`geo-countries`) | [mledoze/countries] (ODbL) + [Natural Earth 110m] (public domain) | ~250 | fill-if-empty `iso_a3` + `numeric_code` + **low-res border `geom`** (→ derived `centroid`/`bbox`). WOF later upgrades `geom` to high-res. |
| `religions` (`religion-scheme`) | [`../religion-presets/taxa.json`](../religion-presets) | 100 | faith taxonomy + theism classifications; re-asserts the migration-seeded tree create-if-absent. |
| `ethnicities` (`ethnicity-scheme`) | CIA World Factbook (public domain), via the tested `internal/hermenea/factbookethnicities` mapper | ~600 | flat, country-linked (the Factbook has no hierarchy/language linkage). `sourceVersion` = git tree SHA. |

**Not generated** (hand-curated, not machine-derivable from a single upstream — left untouched by the
generator): `ranks.yaml` (UA + US per branch, NATO STANAG-2116 grades) and `colors.yaml` (per-domain
palettes for the new rank/religion/ethnicity/country domains).

## Format

Each preset is a small YAML manifest header (`preset` / `objectType` / `source` / `sourceVersion` /
`license` / `dependsOn`) followed by one JSON object per record under `records:`. JSON is a strict subset
of YAML, so the pinax loader parses the records unchanged and the files stay compact + diffable.

[mledoze/countries]: https://github.com/mledoze/countries
[Natural Earth 110m]: https://github.com/nvkelso/natural-earth-vector
