# Module: finance

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [roadmap-decisions](../architecture/roadmap-decisions.md)
> Table prefix: `oikumenea.finance_*`

## Purpose

Holds **bank accounts and payment cards** as authoritative, operator-asserted **directory** data
(D-Finance) — a person (or company) holds accounts, each account is held at a **bank**, and cards hang
off an account. Like [document](document.md) personal codes, the sensitive identifiers (IBAN, card PAN)
are **envelope-encrypted at rest** and holder-scoped; like [vehicle](vehicle.md) registrations,
ownership is a **polymorphic, temporal Link** so joint and corporate accounts and account transfers are
first-class. A **bank is not a new entity** — it is an existing `company`-domain
[tenant](tenant.md) `Organization` (M21 [company](company.md) / M41 / D-UnifiedOrgGraph) that an
account references as its holding institution. This is **first-party, authoritative** data, not an
OSINT overlay — it carries **no** `source`/`confidence` attribution. Scope is a **directory of accounts
and cards**; account balances/transactions are **out of scope** (this is not a payments ledger, DS-55).

## Entities & aggregates

- **Reference catalogs** (`code` + translatable `name`, instance-extensible, D-Code/D-i18n):
  - `finance_account_types` (`19,1,3`) — `current`/`savings`/`deposit`/`loan`/… .
  - `finance_card_networks` (`19,1,4`) — `visa`/`mastercard`/`amex`/… .
- **Objects:**
  - `finance_accounts` (`19,1,1`): the bank account. `institution_id` → a `company`-domain
    `tenant_organizations` row (the bank); the **IBAN** as an **envelope-encrypted** value
    (`iban_ciphertext` / `iban_wrapped_dek` / `key_ref` / `iban_blind_index` — the
    `document_personal_codes` shape, `pii:sensitive`, blind index **unique among active**); `currency`
    (ISO 4217); `account_type_id` FK → catalog; `status`; soft-delete. The **RID is the external
    handle** (no separate `code`).
  - `finance_cards` (`19,1,2`): a payment card. `account_id` → `finance_accounts` (**structural
    containment FK**, CASCADE — like `OrderItem`→`Order`, **not** a reified Link); the full **PAN**
    envelope-encrypted (`pan_ciphertext`/`pan_wrapped_dek`/`key_ref`/`pan_blind_index`, `pii:sensitive`)
    with the display-only clear `bin CHAR(6)` + `last_four CHAR(4)`; `network_id` FK → catalog;
    `card_type` TEXT+CHECK ∈ {`debit`,`credit`}; nullable `expiry_month`/`expiry_year`; optional
    nullable `cardholder_person_id` → `person_persons`; soft-delete. **No CVV/CVC column exists** (see
    *Invariants*).
- **Reified Link** (D-Ontology):
  - `finance_account_holders` (`link__held_by`, `19,2,1`): the **ownership** edge — `account_id` →
    account; a **polymorphic holder** `holder_kind ∈ {person,company}` + `holder_id` (text, no FK —
    F-014, the RID self-describes; mirrors [vehicle](vehicle.md) `registered_to` /
    [company](company.md) `owns_stake`); `role ∈ {primary,joint,authorized_signer}`; **temporal**
    (`effective_from`/`effective_to`). Person-holder rows are `pii:basic`.

## Data model

One schema `oikumenea`, prefix `finance_*`; RID PKs via `new_id(19,kind,type)` with a `*_rid_shape`
CHECK; `set_updated_at()` trigger; soft-delete `deleted_at`; `TEXT`+`CHECK` enums. Encrypted columns
follow the [document](document.md) `document_personal_codes` layout exactly (ciphertext + wrapped DEK +
`key_ref` + keyed-HMAC blind index; D-CryptoProvider) — no new crypto machinery. A card→account is a
plain containment FK (a card belongs to exactly one account). Account/card reads are holder-scoped
(person holders) or company-scoped (corporate accounts); reference catalogs are instance-global.
RLS: person-held finance rows gate through the holder (like documents); no bespoke RLS beyond the
holder-scope gate. Migration `0010_finance_overlays` (M44, built).

## Conjure API surface

`FinanceService` (`/finance/v1`): catalog reads + instance-scope upserts (account-types / card-networks);
`createAccount`/`listAccounts`/`getAccount`/`updateAccount`/`deleteAccount` (IBAN encrypted on write,
decrypted on read for authorized callers); `listAccountHolders`/`addAccountHolder`/`endAccountHolding`
(the polymorphic holder edge); `listCards`/`addCard`/`getCard`/`updateCard`/`deleteCard` (PAN encrypted;
BIN/last-4 returned in clear); and the read-only `listPersonAccounts` (`GET /persons/{id}/accounts`).
Translatable catalog names are returned as a `locale → text` map (D-i18n).

## Dependencies

- [company](company.md) (M21) / [tenant](tenant.md) (M41) — the bank is a `company`-domain
  `tenant_organizations` row; a company can also be an account holder.
- [person](person.md) (M5) — a person account holder / named cardholder.
- `pkg/crypto` (D-CryptoProvider) + `pkg/personalcode` (D-PersonalCodes) — envelope encryption + the
  IBAN (ISO 13616 mod-97) / PAN (Luhn + BIN→network) validators.
- [audit](audit.md) (M1) — every write records an Action; [localization](localization.md) (M2) —
  catalog name maps.

## Authorization touchpoints

Defines/gates `finance.read` (read accounts/cards/catalogs), `finance.manage` (create/update accounts,
cards, holder edges), and the instance-scope `finance.catalog.manage` (account-type / card-network
catalogs). Account and card reads are **scoped through the holder** (D-PersonReadScope) + the shadow
gate for person holders, and via the company for corporate accounts. Holding a bank account never
grants authority (parallel to rank/position — directory data). All writes are audited Actions.

## Invariants & safety

- **IBAN & PAN are `pii:sensitive`**, stored only as ciphertext; the blind index (`pii:none`) enforces
  uniqueness among active rows and enables equality lookup without decryption. Crypto-erased on purge.
- **CVV2/CVC2/CID is never stored** — there is no column for it (PCI-DSS Req 3.2 prohibits storing it
  after authorization, even encrypted).
- Storing the full PAN brings the deployment into **PCI-DSS cardholder-data scope**; the operator
  inherits the applicable obligations (see DS-54 for the out-of-scope alternative).
- A card belongs to exactly one account (`account_id` FK); a card's named `cardholder_person_id` is
  optional and independent of the account's holders.
- On `PersonPurged`, the person's holder edges are erased and any account (with its cards) the person
  **solely** holds is **crypto-erased** (mirrors [document](document.md)); company-held accounts
  survive a person purge.

## Open seams / future

- **Facets & dashboards (M58).** [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope) lands filters + a stats endpoint + a console dashboard
  for this module's listable types: `GET /accounts/stats` and `GET /cards/stats` over `institutionId`/`currency`/`accountTypeId`/`status` and `networkId`/`cardType`/`status`. **No facet touches the IBAN or PAN** — both are envelope-encrypted and in PCI-DSS CDE scope (D-ObjectFacets rule 1). Plus the module's first ontology-registry entry.
  Facets and proposed charts are catalogued in [facets.md](../architecture/facets.md).

- **DS-54** — a **BIN+last-4-only** mode (never persist the full PAN) so a deployment stays **out of
  PCI-DSS scope**, for operators who don't need the full number.
- **DS-55** — account **balances / transactions** (a real financial ledger) — explicitly out of scope
  here; this module is a directory of accounts, not a payments system.
- A `finance` connector target (open-banking / registry feeds) via [hermenea](hermenea.md), once a
  lawful source exists (mirrors DS-45 for companies).
- The `PersonPurged` event-subscriber wiring (shared with document/vehicle/religion) once the bus
  carries it.
