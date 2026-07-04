---
name: reference-atlas-sum-chained-hash
description: atlas.sum uses a chained hash — editing migration N rewrites the hash lines for N..end; that is expected, not canary "churn"
metadata:
  type: reference
---

`migrations/atlas.sum` is a **chained/cumulative** checksum file, not per-file-independent. Each
file's `h1:` line depends on the preceding files, so **editing migration N legitimately rewrites the
hash lines for file N and every file after it** (plus the top total `h1:`). Files *before* N are
untouched.

Consequence: after editing a shipped migration, run `atlas migrate hash --dir "file://migrations"`
and the diff touching later files is **correct**, not corruption. Verify with
`atlas migrate validate --dir "file://migrations"` (empty output = OK).

This corrects the framing in [[project-m28-unit-code-lifecycle]] ("local canary atlas churns all 28"):
that "churn" was the chained-hash cascade — M28 edited migration **0000**, the earliest, so every
subsequent line legitimately changed. The local canary atlas (`v1.2.1-*-canary`, `~/go/bin/atlas`) is
consistent with the committed sum (proven: HEAD sum + all-HEAD `.sql` validates clean under it), so a
re-hash does NOT require a different atlas — it just cascades from the earliest edited file.

Editing-a-shipped-migration workflow that works locally: edit `.sql` → `atlas migrate hash` →
`atlas migrate validate` → `scripts/reset-dev-db.sh` (drops `oikumenea` schema incl. Atlas revision
table, replays all migrations, re-seeds bootstrap admin). Postgres runs on `localhost:5432` (dev DB
`postgres`, pw `dev`). No `ExpectedSchemaRevision` bump when editing in place (only when adding a new
migration).

Seeded locales are **4**: `ukr` (default), `eng`, `spa`, `por` (migration 0002). i18n names live in
`oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)`; `LabelsByID`
(localization/service.go) assigns the **default locale (ukr)** the value of the entity's English
`name` column unless an explicit `ukr` translation row overrides it — so seeding correct Ukrainian
requires a `ukr` row, not just relying on the column.
