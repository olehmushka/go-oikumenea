# M44 — Finance module (bank accounts & payment cards)

## Context

M29–M34 (the OSINT person-intelligence cluster) are all built and verified in sequence; M35–M37
remain designed-not-built, and **M44** — the standalone `finance` module — was chosen as the next
milestone to build. It retires the final `todo.md` idea (banks → accounts → cards).

M44 is **already Decided and Designed**: the binding decision [`D-Finance`](docs/architecture/roadmap-decisions.md)
(roadmap-decisions.md:1545) and the module doc [finance.md](docs/modules/finance.md) both exist and
spell out every table, column, RID, endpoint, and invariant. So this milestone starts at the
**Backend** gate of the pipeline (idea→decided→designed→**backend**→migrated→ui→verified) and runs
through Verified. No new design decisions are required; if code and D-Finance disagree, the code is
wrong.

**What it delivers** (RID **service 19**, tables `finance_*`): bank accounts holding an
envelope-encrypted IBAN, payment cards (envelope-encrypted PAN, **no CVV**) contained under an
account, a polymorphic temporal holder link (person|company), and two reference catalogs. A bank is
**not** a new entity — it is an existing `company`-domain `tenant_organizations` row (M21/M41). This
is authoritative first-party data (no M29 `source`/`confidence` attribution).

**Templates to mirror** (do not invent patterns):
- **vehicle** (`internal/vehicle/`, RID svc 17) — the newest full module: raw-pgx repo, polymorphic
  temporal owner link, catalogs, `GET /persons/{id}/vehicles`, `SubscribePersonEvents` re-homing.
  This is the primary structural template.
- **document** (`internal/document/`, svc 10) — envelope encryption of personal codes: the exact
  `Seal`/`Open`/`BlindIndex` usage (`internal/document/application/service.go`), the
  ciphertext/wrapped_dek/key_ref/blind_index column layout (migration `0009`), and
  `ErasePersonRecords` (the crypto-erase-on-purge method).
- **company** (`internal/company/`) — the polymorphic holder in an ownership graph.

## Backend

### 1. RID registry (service 19) — `pkg/rid`
- `pkg/rid/rid.go`: add `SvcFinance = 19` to the service-code block.
- `pkg/rid/registry.go`: add `SvcFinance: "finance"` to `serviceNames`, and the type rows to
  `typeNames`:
  - `{SvcFinance, KindObject, 1}: "account"`, `{…,1,2}: "card"`,
    `{…,1,3}: "account_type"`, `{…,1,4}: "card_network"`
  - `{SvcFinance, KindLink, 1}: "held_by"`
- These MUST match the migration `platform_rid_types` seed exactly — `rid.AssertMatches` fails boot
  on any drift.

### 2. IBAN + PAN validators — `pkg/personalcode`
- Add `validateIBAN` (ISO 13616: strip spaces/upper-case, length 15–34, move first 4 chars to end,
  A→10…Z→35 expansion, mod-97 == 1) and `validatePAN` (Luhn checksum + derive `bin`=first 6,
  `last_four`=last 4; return normalized digit string). Register under scheme keys `"iban"` / `"pan"`
  in `New()`. Follow the existing `validator func(value string) (normalized string, err error)`
  shape and `errors.Join(ErrInvalid, …)` convention. Add unit tests mirroring the existing
  checksum-validator tests. Note the finance module needs `bin`/`last_four` back from the PAN, so
  expose a small helper (e.g. `SplitPAN(normalized) (bin, last4 string)`) rather than overloading
  `Validate`'s `Result`.

### 3. Migration — `migrations/20260601000034_finance.sql`
Next free revision `0034_finance`. Header comment in the house style (see `0033`). Contents:
- `INSERT INTO platform_rid_services (code, module) VALUES (19,'finance');`
- `INSERT INTO platform_rid_types …` the 5 rows above.
- **`finance_account_types`** (`new_id(19,1,3)`) + **`finance_card_networks`** (`new_id(19,1,4)`):
  catalog shape (`code` UNIQUE, i18n `name` via localization store, `status`, `sort_order`,
  `*_rid_shape` CHECK). Seed baseline rows (current/savings/deposit/loan; visa/mastercard/amex/…).
- **`finance_accounts`** (`new_id(19,1,1)`): `institution_id uuid NOT NULL REFERENCES
  tenant_organizations(id)`, the encrypted-IBAN column set copied verbatim from
  `document_personal_codes` (`iban_ciphertext bytea`, `iban_wrapped_dek bytea`, `key_ref text NOT
  NULL`, `iban_blind_index bytea NOT NULL`), `currency text` (ISO 4217, plain — no currency table,
  per M45 exclusion), `account_type_id uuid REFERENCES finance_account_types`, `status`,
  soft-delete, `set_updated_at` trigger, `*_rid_shape` CHECK. Partial-unique index on
  `iban_blind_index WHERE deleted_at IS NULL`. pii COMMENTs (ciphertext=sensitive, blind_index=none).
- **`finance_cards`** (`new_id(19,1,2)`): `account_id uuid NOT NULL REFERENCES finance_accounts(id)
  ON DELETE CASCADE` (structural containment, like OrderItem→Order), PAN envelope column set
  (`pan_ciphertext`/`pan_wrapped_dek`/`key_ref`/`pan_blind_index`), clear `bin char(6)` +
  `last_four char(4)`, `network_id uuid REFERENCES finance_card_networks`, `card_type text CHECK IN
  ('debit','credit')`, nullable `expiry_month`/`expiry_year`, nullable `cardholder_person_id uuid
  REFERENCES person_persons(id)`, soft-delete, trigger, shape CHECK. **NO CVV column** (PCI Req 3.2).
  Partial-unique on `pan_blind_index WHERE deleted_at IS NULL`.
- **`finance_account_holders`** (`new_id(19,2,1)`, `link__held_by`): `account_id` FK,
  `holder_kind text CHECK IN ('person','company')` + `holder_id text NOT NULL` (**no FK** — the RID
  self-describes, F-014, exactly like `vehicle_registrations.owner_id`), `role text CHECK IN
  ('primary','joint','authorized_signer')`, `effective_from`/`effective_to` (temporal), trigger,
  shape CHECK. Consider a partial-unique for one active `primary` holder per account.
- Bump `internal/platform/db/schemaversion.go` `ExpectedSchemaRevision` → `"0034_finance"` (memory:
  must bump with every migration or readiness gate fails).
- Re-hash: `atlas migrate hash` with a **stable** atlas (the local build is a canary that churns all
  sums — see M28/M45 commit gotchas); run against a fresh `oikumenea_test` DROP SCHEMA reset.

### 4. Conjure contract — `api/finance.conjure.yml`
New `FinanceService` (`/finance/v1`, package `oikumenea.finance`, namespace `Finance`). Model after
`api/vehicle.conjure.yml`. Endpoints (per finance.md §"Conjure API surface"):
- Catalogs: `GET/PUT /account-types`, `GET/PUT /card-networks` (names as `map<string,string>` i18n).
- Accounts: `createAccount` / `listAccounts` / `getAccount` / `updateAccount` / `deleteAccount`
  (IBAN plaintext in on write, decrypted out on read for authorized callers; never in list unless
  gated).
- Holders: `listAccountHolders` / `addAccountHolder` / `endAccountHolding`.
- Cards: `listCards` / `addCard` / `getCard` / `updateCard` / `deleteCard` (PAN in on write;
  BIN/last-4 clear on read, full PAN decrypted only on `getCard` for authorized callers).
- `GET /persons/{id}/accounts` → `listPersonAccounts`.
- Conjure errors `Finance:AccountNotFound`, `Finance:CardNotFound`, `Finance:Conflict` (dup
  IBAN/PAN blind index), `Finance:Invalid`, `Finance:HolderInvalid`.
- Regenerate: run the gödel/conjure generation the repo uses (same command that produced
  `internal/conjure/oikumenea/vehicle`).

### 5. Module code — `internal/finance/`
Mirror the vehicle package layout exactly:
- `domain/finance.go` — entities (Account, Card, Holder, catalogs), inputs/updates, sentinel errors,
  the polymorphic holder-kind constants (`HolderPerson`/`HolderCompany`), validators.
- `domain/repository.go` — the `Repository` port (domain owns the interface).
- `adapters/repository.go` — raw-pgx impl over `db.DBTX` (like `vehicle/adapters/repository.go`).
- `application/service.go` — `NewService(pool, repoFor, audit, cipher, codes)` (takes `*crypto.Cipher`
  + `*personalcode.Registry`, exactly like document). Writes run in a tx and record an audit Action;
  IBAN/PAN validated → sealed → blind-indexed on write, decrypted on authorized read; catalog
  upserts; holder-edge add/end; the `ErasePersonAccounts(ctx, personID)` crypto-erase method
  (mirrors `document.ErasePersonRecords` — erases holder edges and crypto-erases accounts+cards the
  person **solely** holds; company-held accounts survive).
- `application/person_merge.go` — `SubscribePersonEvents(bus)` re-homing
  `finance_account_holders SET holder_id=$2 WHERE holder_id=$1 AND holder_kind='person'` (copy the
  vehicle `SubscribeRepoint` shape). The `PersonPurged` event wiring stays **deferred** (as with
  M31/M32) — `ErasePersonAccounts` exists and is exercised directly by the integration test.
- `transport/service.go` — Conjure server impl: map domain errors → `Finance:*` Conjure errors,
  resolve i18n catalog name maps via `locapp.Service`, enforce perms via `pep.Enforcer`.
- `module.go` — `Register(info, pool, audit, loc, enforcer, cipher, codes)` returning
  `*application.Service` (mirror `vehicle/module.go`).

### 6. Authorization perms
Add `finance.read`, `finance.manage`, `finance.catalog.manage` to the code-defined permission
catalog (find where vehicle/`externalorg` perms are declared — the authorization module's permission
registry) and to the appropriate base roles (reader tier gets `finance.read`; a manage tier gets
`finance.manage`; instance plane gets `finance.catalog.manage`). Account/card reads are
holder-scoped (D-PersonReadScope + shadow gate) — reuse the person read-scope helper the way
document does.

### 7. Wire into `cmd/oikumenea/main.go`
After the vehicle registration block (~line 330): `financeSvc, err := finance.Register(info, pool,
auditSvc, locSvc, enforcer, cipher, personalcode.New())` then `financeSvc.SubscribePersonEvents(bus)`.
`cipher` and `personalcode.New()` are already constructed for the document module — reuse them.

## Migrated
Covered by step 3. Verify: fresh `oikumenea_test` DROP SCHEMA reset, `atlas migrate apply` clean
through `0034_finance`, `atlas migrate hash` with stable atlas, `rid.AssertMatches` green at boot.

## UI
- `web/src/app/(dashboard)/finance/page.tsx` — a Finance console (catalogs + accounts list),
  modeled on `web/src/app/(dashboard)/vehicles/`. Add to `Nav` + `messages.ts`.
- A person **Accounts** panel on `persons/[id]` (via `GET /persons/{id}/accounts`), mirroring the
  person vehicles panel. Show masked IBAN / BIN+last-4 for cards; full values only behind the read
  action.
- Regenerate SDKs so web can call the new service: **Go façade** `clients/go/client.go` (add the
  finance client accessor) + **TS SDK** via `scripts/gen-ts-client.sh` (the M27 pipeline). Run its
  `--verify` drift gate. Then `cd web && npx tsc --noEmit && next build`.

## Docs / bookkeeping (same pass)
- `docs/ontology-mapping.md` — add the finance Object/Link type rows (svc 19).
- `docs/milestones.md` **stage board** — flip the M44 row from `designed` to the gates passed;
  ground each `✅` in a real artifact (migration file, web page, D-block). Update the M44 prose note
  if scope shifts.
- `pkg/rid` and the migration seed are the two RID sources of truth — keep them identical.
- Run the docs link-checker snippet from CLAUDE.md after doc edits.

## Verification (end-to-end — the D-Finance exit criteria)
Write `internal/finance/finance_integration_test.go` (model on `vehicle_integration_test.go`) proving:
1. A person holds an account: **IBAN ciphertext at rest holds no plaintext** (query the raw bytea,
   assert the plaintext IBAN string is absent), **blind index present + unique** (second identical
   IBAN → `Finance:Conflict`), **decrypt round-trips** on authorized read.
2. A **joint** second holder is added (holder edge, role=joint).
3. A card under the account: PAN encrypted at rest, **BIN/last-4 clear**, duplicate PAN →
   `Finance:Conflict`.
4. **Person purge** (`ErasePersonAccounts`) **crypto-erases** the solely-held account + its cards
   (row survives, ciphertext/wrapped_dek nulled), while a **company-held** account survives untouched.
5. Catalog reads return `locale → text` name maps.
Plus: `go build ./...`, the finance unit tests (validators + domain), the full integration sweep on
a fresh `oikumenea_test`, and a **live HTTP smoke** on the running server (create account → list →
add card → GET person accounts) using the `/verify` or `/run` skill. Confirm no other module's seed
tests broke after the migration + RID additions.

## Out of scope (parked, per D-Finance)
- **CVV2/CVC2/CID** — never stored, no column (PCI Req 3.2).
- **DS-54** — BIN+last-4-only (out-of-PCI-scope) mode.
- **DS-55** — account balances / transaction ledger.
- ISO-4217 currency catalog (currency is plain TEXT); a hermenea finance connector.
