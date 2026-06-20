# religion-presets — the M22 faith-taxonomy seed

The curated, real-world **faith taxonomy** that migration
[`migrations/20260601000023_religion.sql`](../../migrations/20260601000023_religion.sql) seeds
(D-Religion, refined — see [docs/modules/religion.md](../../docs/modules/religion.md) and
[roadmap-decisions.md](../../docs/architecture/roadmap-decisions.md)).

The **migration is the seed of record** (it ships the `INSERT … SELECT … FROM (VALUES …)` waves +
builds the closure + derives the root `religion_id` + tags theism). This directory is the **reproducible
recipe** that produced those rows, so the set can be regenerated, reviewed, or extended deterministically.

## Files

- `gen-presets.py` — the generator. Holds the curated tree (deep Christianity + the major world
  religions, anchored to **Wikidata QIDs**) as plain data; no external dependencies.
- `taxa.json` — the bundled preset (generated): `{ranks, taxa[], theism[]}`. Provenance + the future
  hermenea-import seam.

## Shape

Each taxon is `(rank, code, name, parent_code|null, wikidata_id|null)`, ordered **parent-first**. Ranks:
`religion → branch → tradition → sub_tradition → denomination` (a faith need not use every level; the
closure carries true depth). **Boundary:** the tree stops at the major historic churches
(denomination rank); a specific *governed instance* (this diocese/parish) is a tenant unit linking to
the nearest taxon, not a taxon itself.

## Regenerate / extend

```bash
# rewrite taxa.json from the curated set in gen-presets.py
python3 deploy/religion-presets/gen-presets.py

# print the migration INSERT block (paste into 20260601000023_religion.sql when the set changes),
# then refresh the Atlas hash + replay to re-verify
python3 deploy/religion-presets/gen-presets.py --sql
atlas migrate hash --env local
```

To add a faith/branch/denomination: append a row to `TAXA` (and, for a root religion, a `THEISM` tag),
keeping it parent-first. `gen-presets.py` validates codes are unique and every parent/theism reference
resolves before emitting.

## Future: hermenea import (open seam)

A later revision may replace the hand-curated set with a **Wikidata SPARQL** pull (the `wdt:P279*`
subclass tree under `Q9174` "religion") reconciled by QID, ingested through the hermenea connector
framework (as Glottolog/WOF are for M18/M16). Documented in `docs/modules/religion.md` → *Open seams*;
not built in M22.
