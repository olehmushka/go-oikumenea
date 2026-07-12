-- 0037 search generalization (review-2026-08 R-21 / D-PersonSearch generalized).
--
-- July's R-06 (D-PersonSearch) made the person directory typeahead index-served: a pg_trgm GIN over a
-- generated search haystack turns an unanchored ILIKE '%q%' from a per-keystroke sequential scan into a
-- bitmap index scan. Five other typeahead/list surfaces kept the un-indexed pattern — language (27,177
-- languoids seeded at first boot by pinax), geo locations, education institutions, education
-- publications/scholarships, and companies. This migration extends the same trigram machinery to each.
--
-- Index strategy (docs/architecture/decisions.md, D-PersonSearch "Generalized"): a multi-column search is
-- served by a SINGLE GIN trigram index over a STORED generated `search_text` haystack — exactly the
-- person_persons pattern. EXPLAIN on 30k synthetic rows settled the shape: a two-index BitmapOr and an
-- EXPRESSION index over `col || ' ' || col` both LOSE to a seq scan, because the planner has no
-- selectivity statistics for an ILIKE (`~~*`) over a bare expression and defaults to ~4% → a seq scan
-- looks cheaper. A STORED column carries real pg_stats, so the planner estimates the true (low)
-- selectivity of a rare substring and uses the GIN index. The cost is one narrow generated column per
-- table (and its presence in the sqlc-generated model structs, which nothing reads) — worth it for a
-- search that is actually index-served. The application splits each filtered list into an unfiltered List
-- query and a trigram Search query so the predicate is never guarded by `(@q = '' OR …)` — which defeats
-- the index under a generic prepared-statement plan.
--
-- Expand-only (L-UpgradeSafe / D-Migrations): additive indexes + one generated column; pg_trgm already
-- exists (migration 0005). PURE DDL, seeds no rows (no app.environment GUC — D-RIDSeeding). Adding the
-- generated column rewrites location_locations once at apply time.

-- pg_trgm makes unanchored ILIKE '%q%' index-servable via GIN. Idempotent — created in 0005.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 1. language_languoids (internal/language SearchLanguoids): STORED haystack over name + glottocode. code
-- is char(8) (8-char glottocodes, no padding); the concat casts it to text. Reference data: no deleted_at,
-- so no partial clause.
ALTER TABLE oikumenea.language_languoids
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(name || ' ' || code)) STORED;
CREATE INDEX language_languoids_search_trgm
  ON oikumenea.language_languoids USING gin (search_text gin_trgm_ops);
COMMENT ON COLUMN oikumenea.language_languoids.search_text IS 'pii:none';

-- 2. location_locations (internal/geo SearchLocationsByText): STORED haystack over the six address columns.
ALTER TABLE oikumenea.location_locations
  ADD COLUMN search_text text GENERATED ALWAYS AS (
    lower(coalesce(locality,'')     || ' ' || coalesce(admin_area_1,'') || ' ' ||
          coalesce(admin_area_2,'') || ' ' || coalesce(street,'')      || ' ' ||
          coalesce(mgrs,'')         || ' ' || coalesce(raw_address,''))) STORED;
CREATE INDEX location_locations_search_trgm
  ON oikumenea.location_locations USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.location_locations.search_text IS 'pii:none';

-- 3. tenant_organizations (shared: internal/education SearchInstitutions AND internal/company
-- SearchCompanies both search org code+name). STORED haystack over code + name; partial on active rows.
-- Both columns are NOT NULL. This column also surfaces in the sqlc model wherever the table is read — an
-- inert extra field (nothing selects it into a domain type).
ALTER TABLE oikumenea.tenant_organizations
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(code || ' ' || name)) STORED;
CREATE INDEX tenant_organizations_search_trgm
  ON oikumenea.tenant_organizations USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.tenant_organizations.search_text IS 'pii:basic';

-- 4. company_org_profiles.short_name (internal/company SearchCompanies): short_name lives on the joined
-- profile table, so its match branch is a UNION arm — a GIN on the real column (which carries pg_stats),
-- indexed independently of the org haystack.
CREATE INDEX company_org_profiles_short_name_trgm
  ON oikumenea.company_org_profiles USING gin (short_name gin_trgm_ops) WHERE deleted_at IS NULL;

-- 5. education reference (internal/education SearchPublications/SearchScholarships): STORED haystack per
-- catalog over its two matched columns; partial on active rows. All four columns are NOT NULL.
ALTER TABLE oikumenea.education_publications
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(code || ' ' || title)) STORED;
CREATE INDEX education_publications_search_trgm
  ON oikumenea.education_publications USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.education_publications.search_text IS 'pii:none';
ALTER TABLE oikumenea.education_scholarships
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(code || ' ' || name)) STORED;
CREATE INDEX education_scholarships_search_trgm
  ON oikumenea.education_scholarships USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.education_scholarships.search_text IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0037_search_generalization', applied_at = now() WHERE singleton;
