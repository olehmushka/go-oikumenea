-- Migration 0016_person_date_of_death: the M12 date-of-death bio field (D-PersonBio amendment).
--
-- D-PersonBio's M12 amendment mandates a nullable `date_of_death DATE` on person_persons, alongside
-- `birthdate`/`sex`/`country_of_birth`. Death is a BIO ATTRIBUTE, not a lifecycle state: it does NOT
-- transition `status` to `deactivated`/`purged` (a deceased person stays an active directory record;
-- status is orthogonal). It is a full-precision calendar DATE (not a TIMESTAMPTZ instant) — like
-- `birthdate`, partial/approximate dates ride the parked DS-38 seam. The column is `pii:basic` and is
-- on the person purge erasure list (NULLed by PurgePerson with the other bio fields).
--
-- Expand-only: a single additive nullable column. No data change, no destructive op, no backfill.

ALTER TABLE oikumenea.person_persons ADD COLUMN date_of_death date;  -- nullable bio date (a DATE, not an instant)
COMMENT ON COLUMN oikumenea.person_persons.date_of_death IS 'pii:basic';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0016_person_date_of_death', applied_at = now() WHERE singleton;
