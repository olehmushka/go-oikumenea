# Language presets (M18 / D-Languages)

Bundled, opt-in reference-data snapshots loaded by the **hermenea** companion over
`POST /import/{objectType}` (never by a migration). Regenerate reproducibly with
[`gen-presets.py`](gen-presets.py).

| File | Object-type | Records | Source | License |
|---|---|---|---|---|
| `glottolog-5.3.json` | `language-scheme` | ~27k languoids | [Glottolog 5.3 as CLDF](https://github.com/glottolog/glottolog-cldf) (Hammarström, Forkel, Haspelmath & Bank) | CC-BY-4.0 |
| `cldr-scripts.json` | `language-scripts` | ~1k language→script links | [Unicode CLDR](https://github.com/unicode-org/cldr) `languageData` (+ ISO 639-3 ↔ 639-1 from [SIL](https://iso639-3.sil.org)) | Unicode-DFS / CC-BY |

Each file is a JSON **array** of canonical records (the degenerate "source == envelope records" case the
`file` connector + in-memory mapper consume). To refresh from a newer upstream release, point the source's
`locator` at the new URL (the `http` connector) or re-run the generator and commit the result.

**Attribution.** If you use these data, cite Glottolog and the Unicode CLDR per their terms; the
attribution above travels in the canonical envelope's `license` field for lineage.

## Regenerate

```bash
# Download from the pinned upstreams and rewrite both JSON files:
python3 deploy/language-presets/gen-presets.py
# Or reuse already-downloaded copies in a cache dir:
GLOTTOLOG_CACHE=/tmp python3 deploy/language-presets/gen-presets.py
```
