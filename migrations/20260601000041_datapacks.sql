-- 0041 data packs (M54 — D-DataPacks). The D-Pinax boot autoseeder generalizes from purely
-- `go:embed`-bundled presets to ALSO scanning an operator-mounted packs directory (`pinax.packs`),
-- beside the embedded set — same create-if-absent / fill-if-empty / never-delete pipeline, same
-- version gate. This migration records WHERE a seeded preset came from, so an operator can see which
-- pack (if any) supplied a given catalog slice.
--
-- Additive / expand-only (L-UpgradeSafe): one nullable column on the existing marker table. Embedded
-- presets keep `pack` NULL; a mounted pack's presets record the pack's directory name. No data moves,
-- no default backfill needed (NULL == embedded, which is what every existing row already is).
ALTER TABLE oikumenea.pinax_seed_state
  ADD COLUMN pack text;  -- NULL = embedded bundle; else the operator-mounted pack's name (D-DataPacks)

COMMENT ON COLUMN oikumenea.pinax_seed_state.pack IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0041_datapacks', applied_at = now() WHERE singleton;
