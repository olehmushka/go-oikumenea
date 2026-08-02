-- Language module queries (docs/modules/language.md; D-Languages, M18). Read-only lookups over the
-- RID-keyed Glottolog languoid forest + ISO-15924 writing-system registry. The catalog is written by
-- the hermenea import pipeline (language-scheme / language-scripts), not here.

-- name: ListLanguoids :many
-- Languoids in code order, narrowed by the languoid facet set (M58 ticket 4 / D-ObjectFacets: level,
-- family, macroarea, status), the tree-traversal args (immediate parent / top-level-only), and a
-- keyset cursor (after: return rows whose code sorts strictly after it). The page size is clamped by
-- the application. A text query routes to SearchLanguoids instead (review R-21): folding the trigram
-- predicate behind `(@q = '' OR …)` here would defeat the GIN index under a generic plan.
--
-- The four FACET predicates are nargs, not the `sqlc.arg(x)::text = ''` sentinels this query carried
-- before M58 ticket 4: the parity guard reads a facet's narg out of every list AND stats query to
-- prove the dashboard and the list see one world, and a sentinel is invisible to it. The two
-- TRAVERSAL args keep their sentinels — they are not facets and no aggregate counts them.
--
-- has_children flags whether the node has any non-dialect child, so a tree browser can show an expand
-- affordance only where it leads somewhere (family → language; languages whose only children are
-- dialects read as leaves).
SELECT l.id, l.code, l.level, l.name, l.parent_id, l.family_code, l.iso639_3, l.macroarea, l.status,
  EXISTS (
    SELECT 1 FROM oikumenea.language_languoids c
    WHERE c.parent_id = l.id AND c.level <> 'dialect'
  ) AS has_children
FROM oikumenea.language_languoids l
WHERE (sqlc.narg('level')::text IS NULL OR l.level = sqlc.narg('level')::text)
  AND (sqlc.narg('family')::text IS NULL OR l.family_code = sqlc.narg('family')::text)
  AND (sqlc.narg('macroarea')::text IS NULL OR l.macroarea = sqlc.narg('macroarea')::text)
  AND (sqlc.narg('status')::text IS NULL OR l.status = sqlc.narg('status')::text)
  AND (sqlc.arg(parent)::text = '' OR l.parent_id::text = sqlc.arg(parent)::text)
  AND (NOT sqlc.arg(top_level)::bool OR l.parent_id IS NULL)
  AND (sqlc.narg('after')::text IS NULL OR l.code > sqlc.narg('after')::text)
ORDER BY l.code
LIMIT sqlc.arg(lim)::int;

-- name: SearchLanguoids :many
-- The trigram-served twin of ListLanguoids (review R-21 / D-PersonSearch generalized): a non-empty
-- name/glottocode substring, matched against the STORED search_text haystack unconditionally so the
-- single pg_trgm GIN index serves it as a bitmap scan rather than a per-keystroke sequential scan. Same
-- structural filters, projection and keyset as ListLanguoids so the two rows are convertible in the repo.
SELECT l.id, l.code, l.level, l.name, l.parent_id, l.family_code, l.iso639_3, l.macroarea, l.status,
  EXISTS (
    SELECT 1 FROM oikumenea.language_languoids c
    WHERE c.parent_id = l.id AND c.level <> 'dialect'
  ) AS has_children
FROM oikumenea.language_languoids l
WHERE (sqlc.narg('level')::text IS NULL OR l.level = sqlc.narg('level')::text)
  AND (sqlc.narg('family')::text IS NULL OR l.family_code = sqlc.narg('family')::text)
  AND (sqlc.narg('macroarea')::text IS NULL OR l.macroarea = sqlc.narg('macroarea')::text)
  AND (sqlc.narg('status')::text IS NULL OR l.status = sqlc.narg('status')::text)
  AND (sqlc.arg(parent)::text = '' OR l.parent_id::text = sqlc.arg(parent)::text)
  AND (NOT sqlc.arg(top_level)::bool OR l.parent_id IS NULL)
  AND l.search_text ILIKE '%' || sqlc.arg(q)::text || '%'
  AND (sqlc.narg('after')::text IS NULL OR l.code > sqlc.narg('after')::text)
ORDER BY l.code
LIMIT sqlc.arg(lim)::int;

-- name: LanguoidStats :many
-- The languoid dashboard aggregate (M58 ticket 4 / D-ObjectFacets): every selected facet's
-- distribution plus the total, in ONE round-trip and ONE scan. The candidate CTE carries
-- ListLanguoids' STRUCTURAL filter block verbatim, so the list and the dashboard see one world; a
-- branch whose want_* flag is false is skipped by the planner, not merely dropped from the response.
--
-- ONE ARM, and no subject: language_languoids is instance-global reference data — no row-level
-- security, no unit column, no reach predicate. `language.read` held anywhere is the whole gate, so
-- there is no visibility decision for a second arm to make. That is the vehicle / external_organization
-- shape (the ABSENCE of a decision) and NOT the audit ledger's (a decision made entirely by which
-- connection the query runs on); the two are not interchangeable.
--
-- The traversal args (parent / topLevel) have no counterpart here on purpose: they switch the LIST to
-- a one-level hierarchy walk rather than adding a predicate that describes the registry. Neither does
-- the keyset — a page boundary is not a filter.
WITH cand AS MATERIALIZED (
  SELECT l.level, l.family_code, l.macroarea, l.status
  FROM oikumenea.language_languoids l
  WHERE (sqlc.narg('level')::text IS NULL OR l.level = sqlc.narg('level')::text)
    AND (sqlc.narg('family')::text IS NULL OR l.family_code = sqlc.narg('family')::text)
    AND (sqlc.narg('macroarea')::text IS NULL OR l.macroarea = sqlc.narg('macroarea')::text)
    AND (sqlc.narg('status')::text IS NULL OR l.status = sqlc.narg('status')::text)
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'level'::text, c.level::text, count(*)::bigint
FROM cand c WHERE sqlc.arg('want_level')::boolean GROUP BY 2
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint
FROM cand c WHERE sqlc.arg('want_status')::boolean GROUP BY 2
UNION ALL
-- macroarea is SET-VALUED, stored semicolon-joined, and grouped by the LITERAL string rather than
-- unnested: the filter is an exact match, so `Africa;Eurasia` is its own bucket AND its own usable
-- filter value, and the distribution PARTITIONS. Unnesting would double-count and would need the
-- NonPartitioning exemption, which this facet could not legally take anyway — the kernel refuses it
-- when the facet's table IS the listed table, because a row has one value in its own column.
SELECT 'macroarea'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.macroarea::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_macroarea')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
-- family_code is char(8), which pads on read, so the key is rtrim()ed — belt-and-braces rather than a
-- fix. Measured against the shipped catalog: every glottocode is exactly 8 characters, and the ::text
-- cast strips trailing blanks on its own. The rtrim states the intent where that conversion rule
-- would otherwise be load-bearing and invisible.
--
-- 479 distinct families, so the tail MUST be collapsed HERE: the kernel's topNBuckets orders and
-- appends the synthetic buckets but never truncates, so a facet declaring TopN 15 whose SQL emitted
-- every group would render 479 bars.
SELECT 'family'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT rtrim(c.family_code)::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_family')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: LanguoidStatsSearch :many
-- The trigram-served twin of LanguoidStats, exactly as SearchLanguoids is ListLanguoids' twin
-- (R-21): identical filters and BYTE-IDENTICAL aggregates, with the substring predicate added to the
-- candidate set. The repository picks between the two on the SAME branch the list takes, so a
-- searched list and its dashboard cannot end up describing different sets.
WITH cand AS MATERIALIZED (
  SELECT l.level, l.family_code, l.macroarea, l.status
  FROM oikumenea.language_languoids l
  WHERE (sqlc.narg('level')::text IS NULL OR l.level = sqlc.narg('level')::text)
    AND (sqlc.narg('family')::text IS NULL OR l.family_code = sqlc.narg('family')::text)
    AND (sqlc.narg('macroarea')::text IS NULL OR l.macroarea = sqlc.narg('macroarea')::text)
    AND (sqlc.narg('status')::text IS NULL OR l.status = sqlc.narg('status')::text)
    AND l.search_text ILIKE '%' || sqlc.arg(q)::text || '%'
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'level'::text, c.level::text, count(*)::bigint
FROM cand c WHERE sqlc.arg('want_level')::boolean GROUP BY 2
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint
FROM cand c WHERE sqlc.arg('want_status')::boolean GROUP BY 2
UNION ALL
-- macroarea is SET-VALUED, stored semicolon-joined, and grouped by the LITERAL string rather than
-- unnested: the filter is an exact match, so `Africa;Eurasia` is its own bucket AND its own usable
-- filter value, and the distribution PARTITIONS. Unnesting would double-count and would need the
-- NonPartitioning exemption, which this facet could not legally take anyway — the kernel refuses it
-- when the facet's table IS the listed table, because a row has one value in its own column.
SELECT 'macroarea'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.macroarea::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_macroarea')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
-- family_code is char(8), which pads on read, so the key is rtrim()ed — belt-and-braces rather than a
-- fix. Measured against the shipped catalog: every glottocode is exactly 8 characters, and the ::text
-- cast strips trailing blanks on its own. The rtrim states the intent where that conversion rule
-- would otherwise be load-bearing and invisible.
--
-- 479 distinct families, so the tail MUST be collapsed HERE: the kernel's topNBuckets orders and
-- appends the synthetic buckets but never truncates, so a facet declaring TopN 15 whose SQL emitted
-- every group would render 479 bars.
SELECT 'family'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT rtrim(c.family_code)::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_family')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: GetLanguoid :one
SELECT l.id, l.code, l.level, l.name, l.parent_id, l.family_code, l.iso639_3, l.macroarea, l.status,
  EXISTS (
    SELECT 1 FROM oikumenea.language_languoids c
    WHERE c.parent_id = l.id AND c.level <> 'dialect'
  ) AS has_children
FROM oikumenea.language_languoids l WHERE l.id = $1;

-- name: ListWritingSystems :many
SELECT id, code, name, script_type FROM oikumenea.writing_systems ORDER BY code;
