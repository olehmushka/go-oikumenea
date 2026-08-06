-- 0024_facet_enrollment — the LAST M58 type (ticket 7 / D-ObjectFacets): `enrollment`
-- (link__studied_at), the top-level browse over person_education_enrollments plus its dashboard.
--
-- Additive and expand-only: five indexes and four column comments. Nothing is rewritten, nothing is
-- dropped, and no function is added — which is itself the finding of this migration.
--
-- NO NEW REACH FUNCTION, and that is a decision rather than an omission. Ticket 6 added the
-- permission-parameterised `_with` siblings of the 0017 trio because `listAssignments` had always
-- demanded `assignment.read` on a SPECIFIC unit, so trimming it with the generic '%.read' family
-- would have made the endpoint wider. This table asks a different question. An enrollment carries no
-- unit at all: it is scoped THROUGH ITS HOLDER (D-PersonReadScope), and "may this subject read this
-- person" is the generic read-reach question by definition — the endpoint has already checked
-- `education.read` for the education half. So the 0017 trio is exactly right here, and is reused
-- verbatim, the same composition `document_documents` has used since M56 ticket 3.
--
-- Note that person_education_enrollments.unit_id is NOT a visibility column and must never become
-- one. It is the faculty a person studied in — an attribute of the row, and a facet — and gating on
-- it would answer a different question (whose faculty is reachable) from the one the read scope asks
-- (whose person is). The two would agree on most demo data and diverge on exactly the rows that
-- matter: a student enrolled in a faculty nobody reaches, held by a person everybody can read.
--
-- pii: the sweep came back DIRTY for the sixth time in seven tickets. `status` and `effective_from`
-- are both FACETED columns carrying no classification at all, while every neighbouring column on the
-- same table has had one since M20 — so the plaintext guard had nothing to check for two of the seven
-- facets. Corrected in §2; the values are unchanged, only the schema's account of them.
--
-- atlas migrate lint was NOT run: v0.38+ gates it behind Atlas Pro. This migration adds indexes and
-- comments only, so it is additive by construction and there is no destructive change for the gate to
-- find — but the gate did not execute, and saying so is the point.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0007 (person_education_enrollments,
-- education_degree_levels) and 0017 (the reach helpers the scoped arms call).

-- ---------------------------------------------------------------------------------------------------
-- §1. Keyset and grouping indexes for the new browse mode.
--
-- The table has had exactly two indexes since M20 — (person_id) and (institution_id) — because it
-- could only ever be asked "which enrollments does THIS person hold". The new list pages by RID and
-- the dashboard groups by five columns, none of which either index serves.
--
-- All PARTIAL on `deleted_at IS NULL`: enrollments are soft-deleted and both surfaces read only live
-- rows, so the dead ones have no business in the index. institution_id is NOT repeated here — the
-- existing partial index already matches the predicate the filter uses.
--
-- No index on `status`: it is a five-value CHECK set over a table whose rows are overwhelmingly one
-- of them, so a scan beats an index probe at every size the planner will see; the enum facet groups
-- it in the aggregate, which reads the whole candidate set anyway.
-- ---------------------------------------------------------------------------------------------------

-- The keyset index: the list's ORDER BY id under the live-row predicate.
CREATE INDEX IF NOT EXISTS person_education_enrollments_keyset_idx
  ON oikumenea.person_education_enrollments (id) WHERE deleted_at IS NULL;

-- The four grouping columns the dashboard's ref branches GROUP BY, and which the list also filters on
-- exactly. Each is nullable, so each index is genuinely narrower than the table.
CREATE INDEX IF NOT EXISTS person_education_enrollments_program_idx
  ON oikumenea.person_education_enrollments (program_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS person_education_enrollments_unit_idx
  ON oikumenea.person_education_enrollments (unit_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS person_education_enrollments_group_idx
  ON oikumenea.person_education_enrollments (group_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS person_education_enrollments_degree_level_idx
  ON oikumenea.person_education_enrollments (degree_level_id) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------------------------------
-- §2. The pii sweep's findings.
--
-- `status` is an enrollment's lifecycle state (enrolled / graduated / withdrawn / expelled /
-- on_leave) and `effective_from` its intake date. Both are `pii:basic`, NOT `pii:none`: unlike a
-- document's status — which describes the paper — these describe the PERSON's relationship to an
-- institution, and every other column on this table that does so (institution_id, unit_id,
-- degree_level_id, field_of_study) has been `pii:basic` since M20. That tier is also what makes them
-- legal facet sources: D-ObjectFacets rule 2 lets a pii:basic column ride the endpoint's own read
-- code, where anything above it would have to name its field's code.
--
-- effective_to is classified in the same pass for the same reason, though it is not a facet — leaving
-- one half of a date pair unclassified is how the two above were missed.
-- ---------------------------------------------------------------------------------------------------
COMMENT ON COLUMN oikumenea.person_education_enrollments.status IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.effective_to IS 'pii:basic';

-- education_degree_levels.sort_order is the last unclassified column on the catalog the new
-- StrategyCatalog facet drives from. `isced_level`, `code` and `name` were classified in M20; this one
-- was not, and the strategy now reads that table by name, so it should not carry an unaccounted
-- column.
COMMENT ON COLUMN oikumenea.education_degree_levels.sort_order IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0024_facet_enrollment', applied_at = now() WHERE singleton;
