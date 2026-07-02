-- religion-scheme import upserts (D-Religion + D-Pinax, M45). The recursive religion_taxa faith tree
-- (+ per-taxon theism classifications) arrives over POST /import/religion-scheme and is upserted
-- code-keyed, idempotently, non-destructively (mirrors ethnicity-scheme / language-scheme). Records
-- arrive parent-first; the parent code + rank code resolve to RIDs in SQL. The migration already seeds a
-- curated taxonomy (origin='seeded'); the bundled `religions` preset re-runs create-if-absent, so on a
-- boot autoseed the existing rows are skipped and only genuinely-new taxa are inserted. After the batch
-- the closure is rebuilt and each taxon's denormalized root religion_id is re-derived (one tx).

-- name: GetReligionVersion :one
SELECT source_version FROM oikumenea.religion_taxa WHERE code = $1 AND deleted_at IS NULL;

-- name: InsertReligionImport :exec
-- Resolve the rank code to its religion_taxon_ranks RID (falling back to a sentinel uuid so a bad rank
-- code fails the FK loudly), and the parent code to its religion_taxa RID (sentinel fallback so a
-- forward reference fails RESTRICT rather than silently NULLing — records must arrive parent-first).
INSERT INTO oikumenea.religion_taxa (
  code, name, description, rank_id, parent_id, wikidata_id, icon, sort_order, source, source_version, origin)
SELECT sqlc.arg(code)::text,
       sqlc.arg(name)::text,
       NULLIF(sqlc.arg(description)::text, ''),
       COALESCE((SELECT r.id FROM oikumenea.religion_taxon_ranks r
                 WHERE r.code = sqlc.arg(rank_code)::text AND r.deleted_at IS NULL),
                '00000000-0000-0000-0000-000000000000'::uuid),
       CASE WHEN NULLIF(sqlc.arg(parent_code)::text, '') IS NULL THEN NULL
            ELSE COALESCE((SELECT p.id FROM oikumenea.religion_taxa p
                           WHERE p.code = sqlc.arg(parent_code)::text AND p.deleted_at IS NULL),
                          '00000000-0000-0000-0000-000000000000'::uuid) END,
       NULLIF(sqlc.arg(wikidata_id)::text, ''),
       NULLIF(sqlc.arg(icon)::text, ''),
       sqlc.narg(sort_order)::int,
       sqlc.arg(source)::text,
       sqlc.arg(source_version)::text,
       'seeded'::text;  -- import-path rows are pinax seeded-owned (D-Pinax, M45)

-- name: UpdateReligionImport :exec
UPDATE oikumenea.religion_taxa SET
  name           = sqlc.arg(name)::text,
  description    = NULLIF(sqlc.arg(description)::text, ''),
  rank_id        = COALESCE((SELECT r.id FROM oikumenea.religion_taxon_ranks r
                             WHERE r.code = sqlc.arg(rank_code)::text AND r.deleted_at IS NULL),
                            '00000000-0000-0000-0000-000000000000'::uuid),
  parent_id      = CASE WHEN NULLIF(sqlc.arg(parent_code)::text, '') IS NULL THEN NULL
                        ELSE COALESCE((SELECT p.id FROM oikumenea.religion_taxa p
                                       WHERE p.code = sqlc.arg(parent_code)::text AND p.deleted_at IS NULL),
                                      '00000000-0000-0000-0000-000000000000'::uuid) END,
  wikidata_id    = NULLIF(sqlc.arg(wikidata_id)::text, ''),
  icon           = NULLIF(sqlc.arg(icon)::text, ''),
  sort_order     = sqlc.narg(sort_order)::int,
  source         = sqlc.arg(source)::text,
  source_version = sqlc.arg(source_version)::text
WHERE code = sqlc.arg(code)::text AND deleted_at IS NULL;

-- name: DeleteReligionClassifications :exec
DELETE FROM oikumenea.religion_taxon_classifications
WHERE taxon_id = (SELECT id FROM oikumenea.religion_taxa WHERE code = $1 AND deleted_at IS NULL);

-- name: InsertReligionClassification :exec
-- Tie a taxon to a theism classification, resolving both natural keys. A classification code that does
-- not resolve yields no row (silently dropped — resilient to catalog gaps), not fatal.
INSERT INTO oikumenea.religion_taxon_classifications (taxon_id, classification_id)
SELECT t.id, c.id
FROM oikumenea.religion_taxa t, oikumenea.religion_classifications c
WHERE t.code = sqlc.arg(code)::text AND t.deleted_at IS NULL
  AND c.code = sqlc.arg(classification_code)::text AND c.deleted_at IS NULL
ON CONFLICT (taxon_id, classification_id) DO NOTHING;

-- name: ClearReligionClosure :exec
DELETE FROM oikumenea.religion_taxa_closure;

-- name: RebuildReligionClosure :exec
-- Recompute the full transitive closure (mirrors the migration's bulk build): reflexive (t,t,0) plus
-- every ancestor→descendant pair down the tree. Run after ClearReligionClosure, once at import end.
WITH RECURSIVE anc AS (
  SELECT id AS ancestor_id, id AS descendant_id, 0 AS depth
  FROM oikumenea.religion_taxa WHERE deleted_at IS NULL
  UNION ALL
  SELECT a.ancestor_id, t.id, a.depth + 1
  FROM anc a
  JOIN oikumenea.religion_taxa t ON t.parent_id = a.descendant_id AND t.deleted_at IS NULL
)
INSERT INTO oikumenea.religion_taxa_closure (ancestor_id, descendant_id, depth)
SELECT ancestor_id, descendant_id, depth FROM anc;

-- name: DeriveReligionRoots :exec
-- Re-derive each taxon's denormalized root religion_id (the ancestor whose parent is NULL). Run after
-- RebuildReligionClosure, once at import end.
UPDATE oikumenea.religion_taxa t
SET religion_id = root.ancestor_id
FROM (
  SELECT c.descendant_id, c.ancestor_id
  FROM oikumenea.religion_taxa_closure c
  JOIN oikumenea.religion_taxa a ON a.id = c.ancestor_id AND a.parent_id IS NULL
) root
WHERE root.descendant_id = t.id;
