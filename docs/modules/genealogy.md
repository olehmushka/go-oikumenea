# Module: genealogy

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [roadmap-decisions](../architecture/roadmap-decisions.md)
> Table prefix: `oikumenea.genealogy_*` · RID service **21** · Milestones **M63** (export), **M64** (import)

## Purpose

Moves **family trees in and out** of the directory in **GEDCOM 5.5.1**, the format MyHeritage,
Ancestry and FamilySearch import and export.
- **Export** (D-GenealogyExport, M63). A recursive walk over the person family graph, gated per hop,
  serialized to a `.ged` file by an asynchronous job.
- **Import** (D-GenealogyImport, M64). An uploaded `.ged` file is parsed into staging, matched against
  existing persons, reviewed by an operator, then committed through the person APIs.

This module **owns no family data**. Kinship, partnerships, places, approximate dates and life events
all live in [person](person.md) / [personprofile](personprofile.md) /
[personsensitive](personsensitive.md) (D-GenealogyModel M60, D-GenealogicalDates M61, D-LifeEvents
M62). Genealogy owns only the **jobs, the files and the import staging**. It is a reader and a
reviewed writer of person data, never a second home for it.

## Entities & aggregates

- **Objects:**
  - `genealogy_export_jobs` (`21,1,1`). One export request.
    - What was asked: `root_person_id`, `requester_principal_id`, `params` (JSONB: direction,
      generations, includeSiblings/Partners/Events/Contact/Special, locale).
    - Progress: `status ∈ {queued, running, done, failed, expired}`, `failure_code`, `lease_until`.
    - The file: the **envelope-encrypted result** (`result_ciphertext` / `result_wrapped_dek` /
      `key_ref`), `expires_at`.
    - Outcome: `truncated`, `person_count`, `placeholder_count`, `special_classes TEXT[]`.
  - `genealogy_import_batches` (`21,1,2`). One uploaded file.
    - What was uploaded: the **envelope-encrypted raw file** (dropped on commit), `uploader_principal_id`.
    - Progress: `status ∈ {uploaded, parsing, parsed, failed, committing, committed, expired}`,
      `commit_cursor`, `expires_at`.
    - Header facts: `source_system`, `gedcom_version`, `charset`.
  - `genealogy_import_individuals` (`21,1,3`). One staged `INDI`.
    - Parsed facts: `xref`, parsed names, sex, genealogical dates, place texts, `events` JSONB,
      `unknown_tags` JSONB.
    - Matching: `candidates` JSONB (person id + score, readable persons only).
    - Review: `decision ∈ {undecided, match, create, skip}`, `matched_person_id`, `committed_person_id`.
  - `genealogy_import_families` (`21,1,4`). One staged `FAM`: partner and child xrefs, a partnership
    status, dates and a place.
- **No reified Link of its own.** Committed relationships are ordinary person-module Links
  (`link__kin_parent_of`, `link__partnered_with`) stamped `source = imported`,
  `source_ref = <batch RID>`.
- **Actions:**
  - Export: `action__request_genealogy_export`, plus an audited download read.
  - Import: `action__upload_genealogy_import`, `action__decide_genealogy_import_row`,
    `action__commit_genealogy_import`.

## Data model

One schema `oikumenea`, prefix `genealogy_*`, RID PKs via `new_id(21,1,n)`; `set_updated_at()`
triggers; `TEXT` + `CHECK` enums.
- **Encrypted payloads.** Exported files and uploaded raw files are stored in the `document_personal_codes`
  encrypted layout (ciphertext + wrapped DEK + `key_ref`; D-CryptoProvider). There is no blind index,
  since nothing is ever looked up by content.
- **Retention.**
  - Export rows are **hard-deleted** at `expires_at` (`genealogy.export.ttl`, default 24h). Deleting a
    row destroys its wrapped DEK, so expiry is a crypto-erase.
  - Import batches expire at `genealogy.import.ttl` (default 30 days) unless committed. A committed
    batch drops its raw file and keeps its staging rows as provenance.
- **Purge sweep.** No scheduler: expired rows are deleted opportunistically on each create, and lazily
  when read.
- **Size bounds.** Export: 10 generations / 5,000 persons. Import: 20 MB / 20,000 individuals per batch.
- **No RLS.** Visibility is per requester (a job or batch is visible only to the principal that made it,
  plus instance admins), enforced in the application layer.

## Conjure API surface

`GenealogyService` (`/genealogy/v1`), in `api/genealogy.conjure.yml`.

- **Export:**
  - `createExport` — `POST /genealogy/exports`, body `{rootPersonId, direction, generations,
    includeSiblings, includePartners, includeEvents, includeContact, includeSpecial, locale?}`,
    returns the job.
  - `createSelfExport` — `POST /me/genealogy/exports`, same body minus `rootPersonId`; the root is the
    caller's linked person.
  - `listExports` — `GET /genealogy/exports`, the caller's own jobs.
  - `getExport` — `GET /genealogy/exports/{id}`.
  - `downloadExport` — `GET /genealogy/exports/{id}/file`, a Conjure `binary` response
    (`text/vnd.familysearch.gedcom`).
- **Import:**
  - `createImport` — `POST /genealogy/imports`, a `binary` body.
  - `listImports` / `getImport`.
  - `listImportIndividuals` — `GET /genealogy/imports/{id}/individuals`, paged, with candidates.
  - `decideImportIndividual` — `PUT /genealogy/imports/{id}/individuals/{rowId}/decision`.
  - `commitImport` — `POST /genealogy/imports/{id}/commit`.

Translatable labels (event-type names, place names) in staging views come back as `locale → text` maps
(D-i18n). The export picks **one** locale for the file, because GEDCOM carries one string per value.

## Dependencies

- [person](person.md) / [personprofile](personprofile.md) / [personsensitive](personsensitive.md):
  read ports for persons, names, kinships, partnerships, events, addresses and (decrypting)
  religious events; write APIs for the import commit; `ReadablePerson` (D-PersonReadScope).
- [location](location.md) / `geo_places` (D-GeoPlaces): place names and their containment chain for
  `PLAC`.
- [search](search.md): the trigram person search (R-21) for import match candidates.
- [authorization](authorization.md): the PEP, re-run as the requester inside the job.
- `pkg/events` outbox (D-EventOutbox): wakes the job runner.
- `pkg/crypto` (D-CryptoProvider): result and raw-file encryption.
- [audit](audit.md): every request, download, decision and commit.
- [identity-federation](identity-federation.md): account → person for the self-service root.

## Authorization touchpoints

**Export.** `genealogy.export` (new `genealogy-exporter` base role) is checked at create, **again at
run time as the requester**, and at download.

What leaves in the file depends on the codes the requester already holds:

| Data | Needs |
|---|---|
| Kinship arm / partnership arm | `person.kinship.read` / `person.partnership.read` |
| Each node | `ReadablePerson` + the shadow gate (unreadable → placeholder; the walk stops there) |
| Events | `includeEvents` + `person.event.read` |
| Addresses (`RESI`) | `includeContact` + `person.address.read` |
| Religious events (`pii:special`) | `includeSpecial` + **`genealogy.export.special`** (in no base role) + `person.religious_event.read` |

**The root must be readable** (D-PersonReadScope has no self exemption); an unreadable root is a 404
at creation. **Self-service** uses the identical rules, with **no kin exception**.

**Import.** `genealogy.import` (upload, review) and `genealogy.import.commit` are **instance-scope
only**, because committed provisional persons are readable only instance-wide. Match candidates are
limited to persons the importer can read. The commit writes through PersonService, so each write
passes that service's own checks and audit.

## Invariants & safety

- **Nothing in the file goes beyond the requester's reach.** The placeholder is the only exception:
  it discloses that a relationship exists and its kind, never identity — no RID, no `_UID`, no
  attributes. This is the single, recorded amendment to D-VisibilityScope.
- **A queued job never outlives a revoked grant**, because authorization is evaluated at run time,
  not at creation.
- **Every stored file is encrypted**, and expiry is a crypto-erase.
- **The walk always terminates.** Ancestry cycles are refused at write time (D-GenealogyModel), and the
  walker keeps a visited set and hard caps.
- **Import never overwrites** an existing person fact, and never writes without a recorded decision.
  An undecided row blocks commit.
- **The commit is idempotent** per staging row (cursor), and a re-run resumes.
- **The audit records which special data classes left the system**, per export.

## Open seams / future

- **GEDCOM 7** (zip, typed same-sex families, better dates) — the same walk, a second writer.
- **JSON tree** — a native Conjure tree response for the SDKs.
- **Console pedigree / descendant chart** — an interactive tree view over the same walk.
- **Media** (photos, scanned records; `OBJE`) — blocked on DS-39 blob storage.
- **ANSEL** decoding for legacy GEDCOM files.
- **Julian / dual dating** (`@#DJULIAN@`) — parked with D-GenealogicalDates.
- **Online genealogy connectors** (FamilySearch API and similar) as [hermenea](hermenea.md) pull
  connectors; licensing and terms must be settled first.
- **Full GDPR Art. 15/20 export** (DS-7): self-service tree export covers family-tree portability only.
