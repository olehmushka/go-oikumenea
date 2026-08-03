# Module: company

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [roadmap-decisions](../architecture/roadmap-decisions.md)
> Table prefix: `oikumenea.company_*`

## Purpose

Holds **organizations as first-class legal entities** at **registry grade** (D-Companies) — identity,
legal form, multi-jurisdiction registration, industry classification, locations, employment positions,
and the **ownership/affiliation graph** (founders, shareholders, beneficial owners, parent/subsidiary,
succession, branches). The point is *further linking*: people and companies join one queryable graph
(employer, founder, owner, ultimate beneficiary) — the YouControl-style value the user asked for. A
company is **external reference data** the service references but does **not** govern: it is deliberately
**distinct from the deploying org's [tenant](tenant.md) units** (no PDP, no shadow visibility) and
**independent of [education](education.md)** (no shared organization foundation — the user chose
independent modules). It reuses the shared M19 [location](location.md) for addresses and the RID-keyed
`geo_countries` registry for jurisdiction. Like rank/position, none of these grant authorization — they
are directory data. Scope is **structural only**; volatile registry intelligence (financials, court,
tax, sanctions/PEP) is **parked** (DS-45/46/47).

> **M41 / D-UnifiedOrgGraph.** A company is now a **`company`-domain `tenant_organization`** (the org
> *is* the legal entity) plus a **`company_org_profiles`** sidecar keyed by the org RID — mirroring
> [education](education.md)'s institution=org and [religion](religion.md)'s `religion_org_profiles`. The
> former standalone `company_companies` table and its own `15,1,1` object RID are gone. `company` is a
> **reference domain** (`pdp_scoped=false`): instance-global, public reads, app-permission writes, no
> reach-RLS, and (unlike education) **no per-org unit graph** — companies have no internal unit tree. The
> registry child tables (registrations, positions, locations, the ownership-graph links) keep their
> `company_id` columns, now FK → `tenant_organizations`.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md), RID service **15**) —
**Objects:** `Company`, `CompanyPosition` (a billet that **exists while vacant**), `Registration`
(a company-held per-scheme identifier, mirrors the [document](document.md) `PersonalCode`), and the
catalogs `CompanyLegalForm` / `CompanyRegistrationScheme` / `CompanyIndustryClass`.
**Links:** the temporal `link__holds_company_position` (appointment); the ownership/affiliation links
`link__founded`, `link__owns_stake`, `link__beneficiary_of`, `link__succeeded_by`, `link__branch_of`;
and the structural reified links `link__has_industry` (industry assignment) and `link__located_at`
(company ↔ M19 location). **Actions:** create/update/delete, add/remove, fill/end, record — each
audited, `action__<type>` RID (company service 15).

- **Company** (aggregate root) — a `code` (unique among active), translatable `legal_name`, plain
  `short_name`, `legal_form_id`, `ownership_category ∈ private|public|state_owned|municipal|foreign|
  mixed` (an axis **orthogonal** to legal form), optional `country` → `geo_countries`, founded/dissolved
  dates, `state ∈ active|dissolved|merged`. Soft-delete.
- **Registration** — a company's identifier under a registration scheme (LEI / national number / VAT …);
  `validated` records whether the identifier matched the scheme's regex. Unique per `(scheme, identifier)`.
- **CompanyPosition** — a company-owned billet (CEO/director/employee line; vacant-first); an
  **Appointment** is its one-holder, effective-dated filling (reversible), mirroring [membership](membership.md).

**Polymorphic holders.** A founder (`link__founded`) and a shareholder (`link__owns_stake`) may be a
**person or a company**, carried as `(holder_kind ∈ person|company, holder_id TEXT)` with **no FK** —
the RID self-describes its service/kind (F-014 / D-ResourceIdentifiers). Company-holder shareholding
edges form the **ownership DAG**. A **beneficiary** (`link__beneficiary_of`, UBO) is always a natural
person, so it carries a real `person_id` FK.

The person-referencing rows (`company_appointments`, `company_beneficiaries`, and the person-holder
`company_foundings` / `company_shareholdings`) are **`pii:basic`** and erased on person **purge**
(D-PIITiers), swept in the person purge transaction.

## Data model

Conventions per [conventions.md](../architecture/conventions.md) (RID PKs via `new_id(15,…)`,
`TIMESTAMPTZ`, `set_updated_at`, soft-delete, `TEXT`+`CHECK` enums, `code`/translatable `name`). Full
DDL in `migrations/20260601000022_company.sql`.

**Reference catalogs** (`company_legal_forms`, `company_registration_schemes`,
`company_industry_classes`) — RID PK, `code` (unique active) + translatable `name`, `status`,
`sort_order`; migration-seeded with a starter set, instance-extensible.
`company_legal_forms` adds `abbreviation` + optional `country_id` (NULL = generic form; seeded with
generic LLC/JSC/GmbH/PLC + UA ТОВ/ПАТ/ФОП). `company_registration_schemes` adds
`validator_pattern` (POSIX regex, NULL = accept any) + `is_global` (LEI ISO-17442 = true; seeded
lei/duns/ua-edrpou/vat/us-ein). `company_industry_classes` adds `system ∈ nace|isic|kved` (seeded NACE
sections).

**A company = a `company`-domain `tenant_organization` + a `company_org_profiles` sidecar** (M41 /
D-UnifiedOrgGraph). The legal entity itself is a `tenant_organizations` row (domain=`company`,
`pdp_scoped=false`): its stable `code` and registered `legal_name` are the org's `code` + `name` (created
/renamed through the tenant service). There is **no own `company` object RID** — the org RID (`4,1,6`) is
the company's identity, and `company_org_profiles` is keyed by it.

**`company_org_profiles`** — `company_id` PK → `tenant_organizations` (CASCADE; the org RID, no own RID),
`short_name`, `legal_form_id` (RESTRICT), `ownership_category`, `country_id` → `geo_countries`
(RESTRICT, nullable), `founded_on`/`dissolved_on` DATE, `state`, soft-delete. Companies have **no
internal unit tree** (unlike education) — divisions, if needed, would be additive `tenant_units`. The
registry child tables below carry their original `company_id` column names, now FK → `tenant_organizations`.

**`company_registrations`** — `id`, `company_id` (CASCADE), `scheme_id` (RESTRICT), `identifier`,
`validated`. `UNIQUE (scheme_id, identifier) WHERE deleted_at IS NULL`.

**`company_industry_assignments`** *(Link `link__has_industry`)* — `id`, `company_id` (CASCADE),
`industry_class_id` (RESTRICT), `is_primary`. `UNIQUE (company_id, industry_class_id)` + a partial
unique index enforcing **at most one primary** per company.

**`company_locations`** *(Link `link__located_at`)* — `id`, `company_id` (CASCADE), `location_id` →
`location_locations` (RESTRICT, M19), `role ∈ registered|operating|branch`.

**`company_positions`** — `id`, `company_id` (RESTRICT), `code`, translatable `title`,
`status ∈ active|abolished`, `sort_order`. `UNIQUE (company_id, code) WHERE deleted_at IS NULL`.

**`company_appointments`** *(Link `link__holds_company_position`)* — `id`, `person_id` (CASCADE),
`position_id` (RESTRICT), `status ∈ active|ended`, `effective_from`/`effective_to`. **One holder per
billet:** `UNIQUE (position_id) WHERE status='active' AND deleted_at IS NULL`. `pii:basic`.

**`company_foundings`** *(Link `link__founded`)* — `id`, `company_id` (CASCADE), `holder_kind`,
`holder_id` (TEXT, polymorphic, no FK), `founded_on`.

**`company_shareholdings`** *(Link `link__owns_stake`)* — `id`, `company_id` (CASCADE, the issuer),
`holder_kind`, `holder_id` (TEXT), `stake_pct` numeric(7,4) ∈ [0,100], `effective_from`/`effective_to`.

**`company_beneficiaries`** *(Link `link__beneficiary_of`)* — `id`, `company_id` (CASCADE), `person_id`
(CASCADE), `ultimate_pct` numeric(7,4), `declared` (registry-declared vs computed). `pii:basic`.

**`company_successions`** *(Link `link__succeeded_by`)* — `id`, `predecessor_id`, `successor_id`
(both CASCADE companies, `CHECK distinct`), `kind ∈ merger|reorganization|rename|acquisition|spinoff`,
`effective_on`.

**`company_branches`** *(Link `link__branch_of`)* — `id`, `branch_id`, `parent_id` (both CASCADE
companies, `CHECK distinct`). `UNIQUE (branch_id, parent_id) WHERE deleted_at IS NULL`.

## Conjure API surface

`CompanyService` (`/company/v1`, full sketch in `api/company.conjure.yml`):

| Group | Ops | Perm |
|---|---|---|
| Catalogs | `GET/PUT /legal-forms`, `/registration-schemes`, `/industry-classes` | read / `company.catalog.manage` |
| Companies | `POST/GET/PUT/DELETE /companies[/{id}]` (list paginated + `query`) | `company.read` / `company.manage` |
| Registrations | `GET/POST /companies/{id}/registrations`, `PUT/DELETE /registrations/{id}` | `company.read` / `company.manage` |
| Industries | `GET/POST /companies/{id}/industries`, `DELETE /industries/{id}` | `company.read` / `company.manage` |
| Locations | `GET/POST /companies/{id}/locations`, `DELETE /company-locations/{id}` | `company.read` / `company.manage` |
| Positions | `POST/GET/PUT /…positions`, `/abolish`, `/fill`, `POST /appointments/{id}/end` (filter `state=vacant\|filled`) | `company.read` / `company.position.manage` |
| Ownership graph | `GET /companies/{id}/ownership-graph`; `POST` foundings / shareholdings / beneficiaries / successions / branches + their `DELETE` | `company.read` / `company.manage` |
| Person view | `GET /persons/{id}/company-affiliations` (appointments + foundings + shareholdings + beneficiary-of) | `company.read` |

Translatable labels (company `legal_name`, legal-form/scheme/industry `name`, position `title`) return
as a `locale → text` map (assembled via [localization](localization.md) `NamesByID`). Ownership-graph
rows carry best-effort plain `companyLabel`/`holderLabel` (default-locale legal names) for display.
Registration identifiers are validated against the scheme regex; filling a filled billet →
`Company:PositionAlreadyFilled`.

## Dependencies

- **Calls:** [person](person.md) (appointments/beneficiaries CASCADE on person delete),
  [location](location.md) (company location FK), [geo](location.md) (company/legal-form country),
  [localization](localization.md) (name maps), [audit](audit.md) (every write records a `system` Action
  in-transaction).
- **Called by:** [person](person.md) **purge** erases `company_appointments` +
  `company_beneficiaries` + the person-holder `company_foundings` / `company_shareholdings`.
- GLEIF LEI data / national registries (UA EDR) / OpenCorporates can ride the [hermenea](hermenea.md)
  import pipeline via the generic `POST /import/{objectType}` — a future connector, not built in M21.

## Authorization touchpoints

Defines/gates `company.read`, `company.manage`, `company.position.manage`, and the instance-scope
`company.catalog.manage`. Company entities are **instance-global external reference data** — they carry
**no unit scope of their own**, so the PEP satisfies these **anywhere** (like
[location](location.md)/[education](education.md)), and there is **no RLS** on the company tables.
Nothing here is ever an authorization input (D-Rank parallel).

**The COMPANY ROW itself is shadow-gated** (M58 ticket 5). A company IS a `company`-domain tenant
organization plus a sidecar (M41 / D-UnifiedOrgGraph), so it carries that organization's
public/shadow bit and is trimmed by the same rule `listOrganizations` applies — organization reach
DERIVED from unit reach (D-VisibilityScope). This is a correction, not an addition: the gate should
always have been here and was not, so `listCompanies`, `getCompany` and unified **search** handed
shadow organizations to any caller holding `company.read` from M21 until M58 ticket 5. All three now
route through one helper (`gateCompanies` in the transport, `scope.NewOrgScope` for search), and a
gated-out company is `CompanyNotFound` rather than a permission error, because `shadow` hides
existence. The dashboard folds the same predicate into SQL rather than trimming the result — trimming
after the fact is right for a page and wrong for a count.

## Patterns

- **Positions/appointments** mirror [membership](membership.md): vacant-first billets, one active holder
  enforced by a partial-unique index, reversible end (status flip + `effective_to`).
- **Registration-as-scheme-registry** mirrors the [document](document.md) `PersonalCode` /
  `PersonalCodeScheme` pattern (per-scheme validators; LEI is the global spine).
- **Polymorphic holder** (person|company) uses `(holder_kind, holder_id TEXT)` with no FK — the RID
  self-describes (F-014); company existence is validated in the application, person existence is
  best-effort.
- **Person bindings + purge** mirror M18 `person_languages`: company-owned link rows that name a person
  are erased in the person purge transaction.
- **Audit-on-write** (D-Audit): each mutation records one `system`-actor Action (`new_id(15,3,0)`).

## Invariants & safety

- A `code` is unique among active companies; a registration is unique per `(scheme, identifier)`; at
  most one **primary** industry per company.
- **One active appointment per position**; abolishing a filled billet is refused (`Company:InUse`).
- Ownership percentages are constrained to `[0,100]`; a succession/branch cannot point at itself.
- Person-referencing rows are **purge-erased**; companies are never hard-deleted from app code (soft-delete).

## Open seams / future

- **Facets & dashboards (M58).** [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope) lands filters + a stats endpoint + a console dashboard
  for this module's listable types: `GET /companies/stats` over the facets `legalForm`, `ownershipCategory`, `countryId`, `industryClass`, `foundedOn`, `state`; plus the module's first **ontology-registry entry**, retiring the bespoke `"use client"` page that fetches one page of 100 and drops `nextPageToken`.
  Facets and proposed charts are catalogued in [facets.md](../architecture/facets.md).

- **Volatile registry intelligence** — financials, court cases, tax debt, sanctions/PEP flags (**DS-45**,
  connector-fed); company web-domain/contact channels (**DS-46**) — deliberately out of scope.
- **Ownership-graph closure + computed-UBO traversal** (**DS-47**): only one-hop neighbourhoods and
  *declared* beneficiaries are stored; transitive ownership/effective-control computation is deferred.
- **National-registry connectors** (GLEIF LEI, UA EDR, OpenCorporates) over [hermenea](hermenea.md) — the
  import object-type registration is a follow-up.
- **University-as-legal-entity** tie to [education](education.md) — deliberately deferred (independent
  modules; a later cross-link seam).
