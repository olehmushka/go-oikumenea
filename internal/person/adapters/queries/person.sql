-- Person module queries (docs/modules/person.md). The directory aggregate (person_persons), its
-- per-person name variants, and the temporal citizenship/residence links. RID PKs default at the
-- database. The reversible deactivate -> purge lifecycle is the PII-erasure path: purge NULLs every
-- PII column and hard-deletes child rows, keeping the id as a tombstone (audit history references it).
-- A NULL narg leaves the stored value unchanged on update (COALESCE); `code` is immutable.

-- ============================ persons ============================

-- name: InsertPerson :one
-- Create a person. attributes defaults to '{}'; country_of_birth/locale are validated by FKs. Rank is
-- NOT set here — a person holds one rank per rank system via person_ranks (UpsertPersonRank, D-Rank).
INSERT INTO oikumenea.person_persons (
  code, display_name, title, given, given2, surname, surname_prefix, surname2,
  generation, credentials, preferred, birthdate, date_of_death, sex, country_of_birth_id, attributes
) VALUES (
  sqlc.narg('code'), @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), sqlc.narg('birthdate')::date,
  sqlc.narg('date_of_death')::date, @sex,
  sqlc.narg('country_of_birth_id'), COALESCE(sqlc.narg('attributes')::jsonb, '{}'::jsonb)
)
RETURNING *;

-- name: GetPerson :one
SELECT * FROM oikumenea.person_persons WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePerson :one
-- Partial update: a NULL narg leaves the value unchanged. country_of_birth/birthdate/date_of_death
-- cannot be cleared to NULL via this path (open seam). `code` is immutable; rank is set via the
-- person_ranks path (UpsertPersonRank / ClearPersonRank).
UPDATE oikumenea.person_persons SET
  display_name     = COALESCE(sqlc.narg('display_name'), display_name),
  title            = COALESCE(sqlc.narg('title'), title),
  given            = COALESCE(sqlc.narg('given'), given),
  given2           = COALESCE(sqlc.narg('given2'), given2),
  surname          = COALESCE(sqlc.narg('surname'), surname),
  surname_prefix   = COALESCE(sqlc.narg('surname_prefix'), surname_prefix),
  surname2         = COALESCE(sqlc.narg('surname2'), surname2),
  generation       = COALESCE(sqlc.narg('generation'), generation),
  credentials      = COALESCE(sqlc.narg('credentials'), credentials),
  preferred        = COALESCE(sqlc.narg('preferred'), preferred),
  birthdate        = COALESCE(sqlc.narg('birthdate')::date, birthdate),
  date_of_death    = COALESCE(sqlc.narg('date_of_death')::date, date_of_death),
  sex              = COALESCE(sqlc.narg('sex'), sex),
  country_of_birth_id = COALESCE(sqlc.narg('country_of_birth_id'), country_of_birth_id),
  attributes       = COALESCE(sqlc.narg('attributes')::jsonb, attributes)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPersons :many
-- Keyset pagination over the time-ordered RID (an empty cursor starts at the beginning), narrowed by
-- the person facet set (M56 / D-ObjectFacets). The INSTANCE-ADMIN directory list; a scoped caller
-- goes through membership's visibility queries, which carry a byte-identical facet block. The
-- application routes a text query to SearchPersons instead.
--
-- Explicit column list (review R-17): the directory list is a hot path and must NOT hydrate the wide
-- generated search_text haystack (nor the always-NULL deleted_at) — it lists exactly the columns the
-- row->domain mapper reads, so sqlc emits a lean row struct. Keep single-row GetPerson as SELECT *.
SELECT p.id, p.code, p.display_name, p.title, p.given, p.given2, p.surname, p.surname_prefix, p.surname2,
  p.generation, p.credentials, p.preferred, p.birthdate, p.date_of_death, p.sex, p.country_of_birth_id,
  p.attributes, p.status, p.deactivated_at, p.purge_after, p.created_at, p.updated_at
FROM oikumenea.person_persons p
WHERE p.deleted_at IS NULL AND (@after = '' OR p.id::text > @after)
  AND (sqlc.narg('sex')::text IS NULL OR p.sex = sqlc.narg('sex')::text)
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('birthdate_from')::date IS NULL OR p.birthdate >= sqlc.narg('birthdate_from')::date)
  AND (sqlc.narg('birthdate_to')::date IS NULL OR p.birthdate <= sqlc.narg('birthdate_to')::date)
  AND (sqlc.narg('country_of_birth_id')::uuid IS NULL OR p.country_of_birth_id = sqlc.narg('country_of_birth_id')::uuid)
  AND (sqlc.narg('rank_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL
          AND pr.rank_id = sqlc.narg('rank_id')::uuid))
  AND (sqlc.narg('has_account')::boolean IS NULL
       OR sqlc.narg('has_account')::boolean = EXISTS (
            SELECT 1 FROM oikumenea.account_accounts ac
            WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND (sqlc.narg('filter_unit_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          -- The unit itself plus its closure descendants, as an UNCORRELATED set: this subquery
          -- reads only the two facet parameters, never p, so the planner evaluates it once and
          -- probes it as a hash. The correlated form (a closure lookup per candidate person)
          -- measured 242 ms at 10^6 persons against 33 ms for this shape, because it re-walked the
          -- closure for every row the keyset scan touched to fill a page.
          AND fm.unit_id IN (
            SELECT sqlc.narg('filter_unit_id')::uuid
            UNION
            SELECT c.descendant_id
            FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = sqlc.narg('filter_unit_id')::uuid
              AND g.deleted_at IS NULL
              AND g.is_authority_bearing
              AND (sqlc.narg('filter_graph')::text IS NULL OR g.code = sqlc.narg('filter_graph')::text))))
ORDER BY p.id
LIMIT @lim;

-- name: SearchPersons :many
-- Trigram directory search (review R-06). @query is a non-empty case-insensitive substring; a person
-- matches on its own search haystack (display name / code / given / surname) OR any name variant
-- (transliteration / alias) — so a native-script or aka form is findable, not just the Latin display
-- name. The two match branches are a UNION of id sets so EACH stays a GIN trigram bitmap scan (an
-- `A OR EXISTS(subquery)` predicate is NOT indexable and would seq-scan the whole directory); the
-- outer keyset then paginates by RID. Sub-3-char queries fall back to a scan (pg_trgm needs a full
-- trigram) — acceptable for the rare short-prefix case. Lean column list (review R-17): a per-keystroke
-- typeahead path must not hydrate search_text/deleted_at — same lean projection as ListPersons.
SELECT p.id, p.code, p.display_name, p.title, p.given, p.given2, p.surname, p.surname_prefix, p.surname2,
  p.generation, p.credentials, p.preferred, p.birthdate, p.date_of_death, p.sex, p.country_of_birth_id,
  p.attributes, p.status, p.deactivated_at, p.purge_after, p.created_at, p.updated_at
FROM oikumenea.person_persons p
WHERE p.deleted_at IS NULL AND (@after = '' OR p.id::text > @after)
  AND p.id IN (
    SELECT ps.id FROM oikumenea.person_persons ps
      WHERE ps.deleted_at IS NULL AND ps.search_text ILIKE '%' || @query || '%'
    UNION
    SELECT v.person_id FROM oikumenea.person_name_variants v
      WHERE v.search_text ILIKE '%' || @query || '%')
  -- The facet block, byte-identical to ListPersons (M56 / D-ObjectFacets): a text search and a
  -- structural filter compose, and the two admin queries must never disagree about what a facet
  -- selects. The selective trigram set leads, so the facets are applied to a small candidate set.
  AND (sqlc.narg('sex')::text IS NULL OR p.sex = sqlc.narg('sex')::text)
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('birthdate_from')::date IS NULL OR p.birthdate >= sqlc.narg('birthdate_from')::date)
  AND (sqlc.narg('birthdate_to')::date IS NULL OR p.birthdate <= sqlc.narg('birthdate_to')::date)
  AND (sqlc.narg('country_of_birth_id')::uuid IS NULL OR p.country_of_birth_id = sqlc.narg('country_of_birth_id')::uuid)
  AND (sqlc.narg('rank_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL
          AND pr.rank_id = sqlc.narg('rank_id')::uuid))
  AND (sqlc.narg('has_account')::boolean IS NULL
       OR sqlc.narg('has_account')::boolean = EXISTS (
            SELECT 1 FROM oikumenea.account_accounts ac
            WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND (sqlc.narg('filter_unit_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          -- The unit itself plus its closure descendants, as an UNCORRELATED set: this subquery
          -- reads only the two facet parameters, never p, so the planner evaluates it once and
          -- probes it as a hash. The correlated form (a closure lookup per candidate person)
          -- measured 242 ms at 10^6 persons against 33 ms for this shape, because it re-walked the
          -- closure for every row the keyset scan touched to fill a page.
          AND fm.unit_id IN (
            SELECT sqlc.narg('filter_unit_id')::uuid
            UNION
            SELECT c.descendant_id
            FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = sqlc.narg('filter_unit_id')::uuid
              AND g.deleted_at IS NULL
              AND g.is_authority_bearing
              AND (sqlc.narg('filter_graph')::text IS NULL OR g.code = sqlc.narg('filter_graph')::text))))
ORDER BY p.id
LIMIT @lim;

-- ============================ dashboard aggregates (M57) ============================

-- name: PersonStats :many
-- The INSTANCE-ADMIN dashboard aggregate (M57 / D-ObjectFacets): every facet distribution for the
-- directory in ONE round-trip and ONE scan of the candidate set. The candidate CTE carries the facet
-- block VERBATIM from ListPersons — the list and the dashboard must see one world, or a chart would
-- describe a set the list does not return — and each aggregate branch is skipped, not merely hidden,
-- when its want_* flag is false (an unselected or unreadable facet is never grouped).
--
-- No LIMIT anywhere: a count is over the whole filtered set by definition, which is also why this
-- needs no sparse/dense plan dispatch — there is no early termination for a reach set to spoil.
WITH cand AS MATERIALIZED (
  SELECT p.id, p.sex, p.status, p.birthdate, p.country_of_birth_id
  FROM oikumenea.person_persons p
  WHERE p.deleted_at IS NULL
  AND (sqlc.narg('sex')::text IS NULL OR p.sex = sqlc.narg('sex')::text)
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('birthdate_from')::date IS NULL OR p.birthdate >= sqlc.narg('birthdate_from')::date)
  AND (sqlc.narg('birthdate_to')::date IS NULL OR p.birthdate <= sqlc.narg('birthdate_to')::date)
  AND (sqlc.narg('country_of_birth_id')::uuid IS NULL OR p.country_of_birth_id = sqlc.narg('country_of_birth_id')::uuid)
  AND (sqlc.narg('rank_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL
          AND pr.rank_id = sqlc.narg('rank_id')::uuid))
  AND (sqlc.narg('has_account')::boolean IS NULL
       OR sqlc.narg('has_account')::boolean = EXISTS (
            SELECT 1 FROM oikumenea.account_accounts ac
            WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND (sqlc.narg('filter_unit_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          AND fm.unit_id IN (
            SELECT sqlc.narg('filter_unit_id')::uuid
            UNION
            SELECT c.descendant_id
            FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = sqlc.narg('filter_unit_id')::uuid
              AND g.deleted_at IS NULL
              AND g.is_authority_bearing
              AND (sqlc.narg('filter_graph')::text IS NULL OR g.code = sqlc.narg('filter_graph')::text))))
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'sex'::text, c.sex::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_sex')::boolean GROUP BY c.sex
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_status')::boolean GROUP BY c.status
UNION ALL
-- Age in WHOLE YEARS as of today, not the raw date: the bands live in the pkg/facet catalog (one
-- definition, already proven against the DDL), so SQL emits the number and Go assigns the band. A
-- NULL birthdate emits NULL and lands in the mandatory (unknown) bucket.
SELECT 'birthdate'::text,
       (EXTRACT(YEAR FROM age(current_date, c.birthdate)))::integer::text,
       count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_birthdate')::boolean GROUP BY 2
UNION ALL
-- Top-N ref facets collapse their tail into (other) HERE rather than in Go: the tail's sum cannot be
-- known without grouping everything, so it is summed where the rows already are. NULL sorts last in
-- the ranking (`(k IS NULL)` first in the ORDER BY) so the (unknown) bucket never steals a top-N
-- slot from a real value.
SELECT 'countryOfBirth'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.country_of_birth_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_country_of_birth')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
-- rankId is LEFT-joined: a person with no active rank is a real bucket ((unknown)), not a row that
-- silently leaves the distribution. `ord` is the scheme's seniority ordinal — category, then type,
-- then rank — so the chart can be read as a seniority profile instead of a frequency ranking
-- (facets.md ④). A person holds one rank PER SYSTEM, so a multi-system directory counts them once
-- per system: this distribution is per-system by construction.
SELECT 'rankId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, min(t.ord)::bigint
FROM (SELECT g.k, g.n, g.ord, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT r.id::text AS k, count(*) AS n,
                   (rc.sort_order::bigint * 1000000 + rt.sort_order::bigint * 1000 + r.sort_order::bigint) AS ord
            FROM cand c
            LEFT JOIN oikumenea.person_ranks pr ON pr.person_id = c.id AND pr.deleted_at IS NULL
            LEFT JOIN oikumenea.rank_ranks r ON r.id = pr.rank_id AND r.deleted_at IS NULL
            LEFT JOIN oikumenea.rank_types rt ON rt.id = r.type_id AND rt.deleted_at IS NULL
            LEFT JOIN oikumenea.rank_categories rc ON rc.id = rt.category_id AND rc.deleted_at IS NULL
            WHERE sqlc.arg('want_rank_id')::boolean
            GROUP BY 1, 3) g) t
GROUP BY 2
UNION ALL
-- unitId counts ACTIVE memberships, LEFT-joined for the same reason: a person with no active
-- membership is the (unknown) bucket. A person in several units is counted in each, so this
-- distribution deliberately does NOT sum to totalCount — it answers "how many people are in unit X",
-- which is what the chart is read as.
SELECT 'unitId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT m.unit_id::text AS k, count(*) AS n
            FROM cand c
            LEFT JOIN oikumenea.membership_memberships m
                   ON m.person_id = c.id AND m.status = 'active' AND m.deleted_at IS NULL
            WHERE sqlc.arg('want_unit_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'hasAccount'::text,
       (EXISTS (SELECT 1 FROM oikumenea.account_accounts ac
                WHERE ac.person_id = c.id AND ac.deleted_at IS NULL))::text,
       count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_has_account')::boolean GROUP BY 2;

-- name: PersonStatsSearch :many
-- The instance-admin dashboard aggregate under a text search: PersonStats with SearchPersons'
-- trigram id-set leading the candidate CTE. A separate query rather than a nullable @query, for the
-- reason R-21 recorded: a `(@query IS NULL OR search_text ILIKE …)` predicate is not indexable, so the
-- unfiltered dashboard — the common case — would seq-scan the directory every time.
WITH cand AS MATERIALIZED (
  SELECT p.id, p.sex, p.status, p.birthdate, p.country_of_birth_id
  FROM oikumenea.person_persons p
  WHERE p.deleted_at IS NULL
  AND (sqlc.narg('sex')::text IS NULL OR p.sex = sqlc.narg('sex')::text)
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('birthdate_from')::date IS NULL OR p.birthdate >= sqlc.narg('birthdate_from')::date)
  AND (sqlc.narg('birthdate_to')::date IS NULL OR p.birthdate <= sqlc.narg('birthdate_to')::date)
  AND (sqlc.narg('country_of_birth_id')::uuid IS NULL OR p.country_of_birth_id = sqlc.narg('country_of_birth_id')::uuid)
  AND (sqlc.narg('rank_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL
          AND pr.rank_id = sqlc.narg('rank_id')::uuid))
  AND (sqlc.narg('has_account')::boolean IS NULL
       OR sqlc.narg('has_account')::boolean = EXISTS (
            SELECT 1 FROM oikumenea.account_accounts ac
            WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND (sqlc.narg('filter_unit_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          AND fm.unit_id IN (
            SELECT sqlc.narg('filter_unit_id')::uuid
            UNION
            SELECT c.descendant_id
            FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = sqlc.narg('filter_unit_id')::uuid
              AND g.deleted_at IS NULL
              AND g.is_authority_bearing
              AND (sqlc.narg('filter_graph')::text IS NULL OR g.code = sqlc.narg('filter_graph')::text))))
  AND p.id IN (
    SELECT ps.id FROM oikumenea.person_persons ps
      WHERE ps.deleted_at IS NULL AND ps.search_text ILIKE '%' || @query || '%'
    UNION
    SELECT v.person_id FROM oikumenea.person_name_variants v
      WHERE v.search_text ILIKE '%' || @query || '%')
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'sex'::text, c.sex::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_sex')::boolean GROUP BY c.sex
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_status')::boolean GROUP BY c.status
UNION ALL
-- Age in WHOLE YEARS as of today, not the raw date: the bands live in the pkg/facet catalog (one
-- definition, already proven against the DDL), so SQL emits the number and Go assigns the band. A
-- NULL birthdate emits NULL and lands in the mandatory (unknown) bucket.
SELECT 'birthdate'::text,
       (EXTRACT(YEAR FROM age(current_date, c.birthdate)))::integer::text,
       count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_birthdate')::boolean GROUP BY 2
UNION ALL
-- Top-N ref facets collapse their tail into (other) HERE rather than in Go: the tail's sum cannot be
-- known without grouping everything, so it is summed where the rows already are. NULL sorts last in
-- the ranking (`(k IS NULL)` first in the ORDER BY) so the (unknown) bucket never steals a top-N
-- slot from a real value.
SELECT 'countryOfBirth'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.country_of_birth_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_country_of_birth')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
-- rankId is LEFT-joined: a person with no active rank is a real bucket ((unknown)), not a row that
-- silently leaves the distribution. `ord` is the scheme's seniority ordinal — category, then type,
-- then rank — so the chart can be read as a seniority profile instead of a frequency ranking
-- (facets.md ④). A person holds one rank PER SYSTEM, so a multi-system directory counts them once
-- per system: this distribution is per-system by construction.
SELECT 'rankId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, min(t.ord)::bigint
FROM (SELECT g.k, g.n, g.ord, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT r.id::text AS k, count(*) AS n,
                   (rc.sort_order::bigint * 1000000 + rt.sort_order::bigint * 1000 + r.sort_order::bigint) AS ord
            FROM cand c
            LEFT JOIN oikumenea.person_ranks pr ON pr.person_id = c.id AND pr.deleted_at IS NULL
            LEFT JOIN oikumenea.rank_ranks r ON r.id = pr.rank_id AND r.deleted_at IS NULL
            LEFT JOIN oikumenea.rank_types rt ON rt.id = r.type_id AND rt.deleted_at IS NULL
            LEFT JOIN oikumenea.rank_categories rc ON rc.id = rt.category_id AND rc.deleted_at IS NULL
            WHERE sqlc.arg('want_rank_id')::boolean
            GROUP BY 1, 3) g) t
GROUP BY 2
UNION ALL
-- unitId counts ACTIVE memberships, LEFT-joined for the same reason: a person with no active
-- membership is the (unknown) bucket. A person in several units is counted in each, so this
-- distribution deliberately does NOT sum to totalCount — it answers "how many people are in unit X",
-- which is what the chart is read as.
SELECT 'unitId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT m.unit_id::text AS k, count(*) AS n
            FROM cand c
            LEFT JOIN oikumenea.membership_memberships m
                   ON m.person_id = c.id AND m.status = 'active' AND m.deleted_at IS NULL
            WHERE sqlc.arg('want_unit_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'hasAccount'::text,
       (EXISTS (SELECT 1 FROM oikumenea.account_accounts ac
                WHERE ac.person_id = c.id AND ac.deleted_at IS NULL))::text,
       count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_has_account')::boolean GROUP BY 2;

-- name: ListPersonsByIDs :many
-- Load the base person rows for a set of RIDs (the D-PersonReadScope directory-list union resolves
-- visible person ids through memberships, then hydrates the rows here). Ordered by RID so the caller
-- can re-key to its keyset order. Lean column list (review R-17): the scoped-list hydration is a hot
-- path — same lean projection as ListPersons (no search_text/deleted_at).
SELECT id, code, display_name, title, given, given2, surname, surname_prefix, surname2,
  generation, credentials, preferred, birthdate, date_of_death, sex, country_of_birth_id,
  attributes, status, deactivated_at, purge_after, created_at, updated_at
FROM oikumenea.person_persons
WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL
ORDER BY id;

-- name: UpsertPersonRank :one
-- Set the person's rank in ONE rank system (the HOLDS_RANK link; one rank per system — D-Rank). The
-- system is DERIVED in SQL from the rank (rank_ranks.system_id), so the caller passes only person + rank.
-- Selecting FROM rank_ranks means an unknown/soft-deleted rank yields NO row → the repo maps the empty
-- result to ErrUnknownRank (no FK/not-null ambiguity). On an existing active (person, system) row the
-- rank is replaced; a previously cleared (soft-deleted) row is left and a fresh active row inserted.
INSERT INTO oikumenea.person_ranks (person_id, system_id, rank_id)
SELECT @person_id, r.system_id, r.id
FROM oikumenea.rank_ranks r
WHERE r.id = @rank_id AND r.deleted_at IS NULL
ON CONFLICT (person_id, system_id) WHERE deleted_at IS NULL
  DO UPDATE SET rank_id = excluded.rank_id
RETURNING *;

-- name: ClearPersonRank :exec
-- Clear (soft-delete) the person's active rank in one rank system. No-op when none is held there.
UPDATE oikumenea.person_ranks SET deleted_at = now()
WHERE person_id = @person_id AND system_id = @system_id AND deleted_at IS NULL;

-- name: ListPersonRanks :many
-- The person's active ranks, one per system, ordered by the rank system's sort_order (D-RankSystems).
SELECT pr.* FROM oikumenea.person_ranks pr
JOIN oikumenea.rank_systems s ON s.id = pr.system_id
WHERE pr.person_id = @person_id AND pr.deleted_at IS NULL
ORDER BY s.sort_order, pr.system_id;

-- name: DeactivatePerson :one
UPDATE oikumenea.person_persons
SET status = 'deactivated', deactivated_at = now(), purge_after = @purge_after
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ReactivatePerson :one
UPDATE oikumenea.person_persons
SET status = 'active', deactivated_at = NULL, purge_after = NULL
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: PurgePerson :one
-- Hard-erase PII: NULL every pii:basic/contact column, reset sex/attributes, keep the id tombstone.
-- Child rows (incl. person_ranks) are removed separately (DeleteAll*).
UPDATE oikumenea.person_persons SET
  code = NULL, display_name = '', title = NULL, given = NULL, given2 = NULL,
  surname = NULL, surname_prefix = NULL, surname2 = NULL, generation = NULL,
  credentials = NULL, preferred = NULL, birthdate = NULL, date_of_death = NULL, sex = 'not_known',
  country_of_birth_id = NULL, attributes = '{}'::jsonb,
  status = 'purged', deactivated_at = NULL, purge_after = NULL
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAllPersonRanks :exec
DELETE FROM oikumenea.person_ranks WHERE person_id = @person_id;

-- name: DeleteAllNameVariants :exec
DELETE FROM oikumenea.person_name_variants WHERE person_id = @person_id;

-- name: UpsertNameVariant :one
-- Add or replace the canonical TRANSLITERATION variant for (person, locale) — the original one-per-locale
-- name form (D-PersonNamesCLDR). The i18n_locales FK validates the locale. variant_kind is forced to
-- 'transliteration' so the conflict targets the partial unique index; aliases use InsertNameAlias.
INSERT INTO oikumenea.person_name_variants (
  person_id, locale, display_name, title, given, given2, surname, surname_prefix,
  surname2, generation, credentials, preferred, is_primary, variant_kind
) VALUES (
  @person_id, @locale, @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), @is_primary, 'transliteration'
)
ON CONFLICT (person_id, locale) WHERE variant_kind = 'transliteration' DO UPDATE SET
  display_name = excluded.display_name, title = excluded.title, given = excluded.given,
  given2 = excluded.given2, surname = excluded.surname, surname_prefix = excluded.surname_prefix,
  surname2 = excluded.surname2, generation = excluded.generation, credentials = excluded.credentials,
  preferred = excluded.preferred, is_primary = excluded.is_primary
RETURNING *;

-- name: ClearPrimaryNameVariants :exec
-- Demote the person's primary transliteration variant(s) (is_primary marks at most one transliteration).
UPDATE oikumenea.person_name_variants SET is_primary = false
WHERE person_id = @person_id AND is_primary AND variant_kind = 'transliteration';

-- name: DeleteNameVariant :one
-- Delete the TRANSLITERATION variant for a locale (aliases are deleted by id via DeleteNameAlias).
DELETE FROM oikumenea.person_name_variants
WHERE person_id = @person_id AND locale = @locale AND variant_kind = 'transliteration'
RETURNING id;

-- name: ListNameVariants :many
SELECT * FROM oikumenea.person_name_variants WHERE person_id = @person_id
ORDER BY variant_kind, locale, id;

-- name: InsertNameAlias :one
-- Add an ALIAS name form (variant_kind in aka|former_legal|maiden|pseudonym|cover; D-PhysicalIdentity).
-- Aliases are unconstrained per (person, locale) and addressed by their RID; they may carry attribution.
INSERT INTO oikumenea.person_name_variants (
  person_id, locale, display_name, title, given, given2, surname, surname_prefix,
  surname2, generation, credentials, preferred, is_primary, variant_kind, source, confidence
) VALUES (
  @person_id, @locale, @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), false, @variant_kind,
  sqlc.narg('source'), sqlc.narg('confidence')
)
RETURNING *;

-- name: DeleteNameAlias :one
-- Delete one alias by its RID (holder-scoped). Refuses the transliteration kind (use DeleteNameVariant).
DELETE FROM oikumenea.person_name_variants
WHERE id = @id AND person_id = @person_id AND variant_kind <> 'transliteration'
RETURNING id;

-- ============================ citizenships ============================

-- name: GetActivePersonByCode :one
-- Look up an active person by their stable `code` (D-Code). Used by identity-federation for
-- just-in-time link-on-match (D-JIT) and the first-admin bootstrap's find-or-create (D-Bootstrap).
SELECT * FROM oikumenea.person_persons
WHERE code = @code AND status = 'active' AND deleted_at IS NULL;

-- ============================ emails (D-PersonContactChannels) ============================

