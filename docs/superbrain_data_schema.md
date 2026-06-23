# Superbrain: people intelligence data schema

Target system: Claude Code  
Purpose: Define collectable data fields, their types, sensitivity levels, sources, and usage notes for a people-intelligence platform.

---

## How to read this document

Each field entry contains:
- **Type** — data type for storage schema (`string`, `string[]`, `date`, `boolean`, `object`, `enum`)
- **Sensitivity** — `standard` | `sensitive` | `special_category` (GDPR Art.9 / HIPAA)
- **Source** — where this data typically comes from
- **Notes** — collection caveats, cross-reference potential, legal flags

---

## Macro-category 1: Physical identity

> Who the person is — body, documents, appearance.  
> Anchor fields for entity resolution and deduplication across datasets.

### 1.1 Legal name

```
type:        string
sensitivity: standard
source:      government documents, court filings, KYC records
```

Full name as it appears on government-issued ID. May differ from commonly used name or alias. Critical join key when matching records across databases — always store alongside aliases and transliterations.

Subfields to consider:
- `first_name`, `last_name`, `middle_name`
- `name_prefix` (Dr., Gen., etc.)
- `name_suffix` (Jr., III, etc.)
- `transliterations[]` — for non-Latin scripts

---

### 1.2 Driver's licence number

```
type:        string
sensitivity: sensitive
source:      DMV records, background check providers, KYC
jurisdiction: per issuing state/country
```

Strong cross-database join key. Tied to address history, vehicle registration, and traffic violations. Store with issuing jurisdiction — the number alone is ambiguous across states/countries.

Cross-references: vehicle records, traffic violations, address history changes.

---

### 1.3 Physical description

```
type:        object
sensitivity: standard
source:      government ID scans, arrest records, surveillance, media
```

Subfields:
- `height_cm` — integer
- `weight_kg` — integer
- `eye_color` — enum
- `hair_color` — enum
- `build` — enum (slim / medium / heavy)
- `distinguishing_marks[]` — string array (tattoos, scars, piercings)

Used for visual identification and cross-referencing with surveillance footage, press photos, or mugshot databases.

---

### 1.4 Ethnicity

```
type:        enum | string
sensitivity: special_category
source:      self-declared, inferred from name/language/geography
legal:       GDPR Art.9 — requires explicit consent or statutory basis
```

Self-identified or inferred. Never infer and store as fact without flagging inference method and confidence. Legally restricted in most jurisdictions — cannot be used as a scoring or filtering criterion in employment, credit, or law enforcement profiling without explicit legal basis.

---

### 1.5 Biometric data

```
type:        object
sensitivity: special_category
source:      government databases, border control, private biometric systems
legal:       GDPR Art.9, BIPA (Illinois), various US state laws
```

Subfields:
- `fingerprint_hash` — never store raw, only hashed/tokenized
- `facial_geometry` — embedding vector from face recognition
- `iris_scan_hash`
- `voiceprint`
- `gait_signature`

Immutable — unlike passwords, biometrics cannot be changed if compromised. Treat as the highest-risk data class in the schema. Store only tokenized representations, never raw biometric data. Access logging mandatory.

---

### 1.6 Blood type

```
type:        enum
sensitivity: sensitive
source:      medical records, military files, forensic databases
values:      A+, A-, B+, B-, AB+, AB-, O+, O-
```

Useful in medical, forensic, and military contexts. Often extracted from service records or insurance filings. Low standalone value; high value when combined with medical history.

---

## Macro-category 2: Location & presence

> Where the person is, was, or can be reached.  
> Some of the highest-signal data in the schema — reveals routines, relationships, and behavior patterns.

### 2.1 Home address

```
type:        object[]
sensitivity: sensitive
source:      property records, voter rolls, utility data, credit bureaus
```

Store as an array — address history is often more valuable than current address. Each entry:
- `street`, `city`, `state`, `postal_code`, `country`
- `valid_from` — date
- `valid_to` — date | null (null = current)
- `source` — string
- `confidence` — float 0–1

Cross-references: property ownership records, mortgage filings, utility accounts, voter registration.

---

### 2.2 Work address

```
type:        object[]
sensitivity: standard
source:      company filings, LinkedIn, building access records, business cards
```

Same schema as home address. Changes with employment — link to employment history timeline. Often semi-public (company headquarters) but can be specific (floor, suite) which is more sensitive.

---

### 2.3 Mailing address

```
type:        object[]
sensitivity: sensitive
source:      postal records, legal filings, subscription data
```

PO boxes, mail forwarding services. Notably, high-profile individuals often use a mailing address specifically to obscure their residential address — the presence of a distinct mailing address is itself a privacy-seeking signal worth flagging.

---

### 2.4 GPS location history

```
type:        object[]
sensitivity: special_category (in many jurisdictions)
source:      mobile device telemetry, vehicle telematics, location data brokers
legal:       requires legitimate basis; GDPR applies; US state laws vary
```

Each entry:
- `lat` — float
- `lng` — float
- `accuracy_m` — integer (meters)
- `timestamp` — datetime (UTC)
- `source` — enum (device / vehicle / wifi / cell_tower)
- `derived_label` — string | null (e.g. "home", "workplace", inferred)

At scale, reveals: daily routines, workplace, home, religious attendance, medical visits, secret relationships, travel. One of the most analytically powerful fields in the schema. Store compressed; query via spatial index.

---

### 2.5 IP address history

```
type:        object[]
sensitivity: sensitive
source:      platform logs, breach data, login records
```

Each entry:
- `ip` — string (IPv4 or IPv6)
- `timestamp` — datetime
- `context` — enum (login / activity / registration)
- `resolved_isp` — string
- `resolved_country` — string
- `is_vpn` — boolean
- `is_tor` — boolean

Cross-reference with breach/leak datasets for account attribution. VPN/Tor flag is a behavior signal. Multiple IPs from same session can indicate credential sharing.

---

## Macro-category 3: Network & relationships

> Who the person knows, trusts, and depends on.  
> Every contact is a graph edge to another person entity.

### 3.1 Emergency contacts

```
type:        object[]
sensitivity: sensitive
source:      HR records, hospital intake forms, insurance filings, government applications
```

Each entry:
- `name` — string
- `relationship` — enum (spouse / parent / sibling / friend / colleague / other)
- `phone[]` — string array
- `email[]` — string array
- `linked_person_id` — UUID | null (link to another entity in the graph)

The `linked_person_id` field is key: this transforms a contact field into a graph edge. Each emergency contact should be resolved against the main entity database and linked if a match exists. Reveals family structure, closest associates, and trust hierarchy — data rarely obtainable from public sources.

---

## Macro-category 4: Financial & assets

> Money flows, ownership, and wealth signals.

### 4.1 Crypto wallets

```
type:        object[]
sensitivity: sensitive (publicly readable, but attribution is sensitive)
source:      blockchain analysis, exchange KYC leaks, on-chain clustering, OSINT
```

Each entry:
- `address` — string
- `chain` — enum (BTC / ETH / SOL / USDT / other)
- `confidence` — float 0–1 (attribution confidence)
- `attribution_method` — enum (self_disclosed / clustering / exchange_leak / osint)
- `first_seen` — date
- `last_seen` — date
- `balance_usd_approx` — float | null

All on-chain transaction data is public and permanently traceable. Wallet clustering algorithms can surface additional attributed addresses from the same entity. Cross-reference with exchange KYC data (from leaks or legal requests) for real-world identity binding. Do not treat inferred wallets as confirmed without confidence threshold.

---

## Macro-category 5: Behavioral & psychological profile

> How the person thinks, decides, and politically aligns.  
> Highest analytical leverage; highest potential for misuse.

### 5.1 Personality type

```
type:        object
sensitivity: sensitive
source:      self-reported surveys, NLP inference from text, HR assessments
```

Subfields per framework:
- `mbti` — enum (INTJ, ENFP, etc.) | null
- `big_five` — object: `openness`, `conscientiousness`, `extraversion`, `agreeableness`, `neuroticism` (each 0.0–1.0)
- `disc` — enum (D / I / S / C) | null
- `enneagram` — integer 1–9 | null
- `inference_method` — enum (self_reported / nlp_text / hr_assessment / social_media)
- `confidence` — float 0–1

NLP-derived personality scores can be generated from writing samples, social media posts, or interview transcripts. Flag inference method and confidence — self-reported data is categorically different from inferred data.

---

### 5.2 Political leaning

```
type:        object
sensitivity: special_category
source:      party registration, donation records, voting history, social media signals
legal:       GDPR Art.9 — political opinions are explicitly listed
```

Subfields:
- `declared_affiliation` — string | null (party name)
- `inferred_spectrum` — float -1.0 (far left) to 1.0 (far right) | null
- `inference_sources[]` — string array
- `donation_records[]` — object array (recipient, amount, date)
- `voting_history_available` — boolean

Distinguish between declared (party registration, public donations) and inferred (content engagement, network analysis). Inferred political leaning without consent is high-risk. Store declared and inferred separately and never merge into a single score.

---

## Macro-category 6: Legal & regulatory exposure

> Criminal, civil, and compliance risk signals.  
> Mostly public record; varies significantly by jurisdiction.

### 6.1 Criminal record

```
type:        object[]
sensitivity: sensitive
source:      court databases, background check providers, PACER (US), national police databases
```

Each entry:
- `offense` — string
- `offense_category` — enum (violent / financial / drug / property / cyber / other)
- `verdict` — enum (convicted / acquitted / dismissed / pending)
- `sentence` — string | null
- `date` — date
- `jurisdiction` — string
- `expunged` — boolean

Distinct from arrest history — a criminal record requires a conviction. Expungement status must be tracked: in many jurisdictions, expunged records cannot be surfaced or used. Jurisdiction-specific law governs what can be stored and displayed.

---

### 6.2 Arrest history

```
type:        object[]
sensitivity: sensitive
source:      police blotters, jail booking records, mugshot databases, court dockets
legal:       restricted use in employment (US Ban-the-Box laws), varies by state/country
```

Each entry:
- `charge` — string
- `arresting_agency` — string
- `date` — date
- `disposition` — enum (convicted / acquitted / dismissed / charges_dropped / pending / unknown)
- `mugshot_available` — boolean
- `jurisdiction` — string

Critical distinction: arrest does not imply guilt. Many arrests result in no charges or acquittal. Displaying arrest history without disposition context is legally and ethically problematic. Always store and display disposition.

---

### 6.3 Court judgments

```
type:        object[]
sensitivity: sensitive
source:      PACER, state court databases, county clerk records, legal data providers
```

Each entry:
- `case_number` — string
- `type` — enum (civil / family / bankruptcy / probate / administrative)
- `parties` — object: `plaintiff`, `defendant`
- `outcome` — string
- `amount_usd` — float | null
- `date_filed` — date
- `date_resolved` — date | null
- `jurisdiction` — string

Civil judgments (debt, property, custody, injunctions) reveal financial stress and interpersonal conflicts outside the criminal context. Public record in most jurisdictions.

---

### 6.4 Regulatory sanctions

```
type:        object[]
sensitivity: sensitive
source:      SEC EDGAR, FCA register, FDA warning letters, FINRA BrokerCheck, regulator APIs
```

Each entry:
- `regulator` — string (e.g. "SEC", "FCA", "FDA")
- `action_type` — enum (fine / ban / cease_and_desist / license_revocation / warning)
- `description` — string
- `amount_usd` — float | null
- `date` — date
- `status` — enum (active / resolved / appealed)
- `source_url` — string

Tied to a person's professional role. Critical for due diligence on executives, fund managers, doctors, lawyers, and other licensed professionals. Many regulators provide structured APIs.

---

### 6.5 OFAC / EU sanctions lists

```
type:        object
sensitivity: sensitive
source:      OFAC SDN list, EU consolidated list, UN sanctions, INTERPOL notices
implementation: LIVE API LOOKUP ONLY — do not store as static data
```

Fields to store per match:
- `on_list` — boolean
- `lists[]` — string array (which lists the person appears on)
- `list_entry_date` — date | null
- `program` — string | null (e.g. "UKRAINE-EO13685")
- `last_checked` — datetime
- `next_check_due` — datetime

**Implementation note:** OFAC and EU lists update daily. Storing a snapshot creates compliance risk — a person removed from the list may still appear as sanctioned in a stale cache. Implement as a live lookup at query time with a short-TTL cache (≤24h). Required for any AML/KYC workflow.

---

## Macro-category 7: Political & institutional ties

> Power structures, affiliations, and influence networks.  
> Mostly public record; high value for due diligence and influence mapping.

### 7.1 Wikipedia page

```
type:        object | null
sensitivity: standard
source:      Wikimedia API
```

Subfields:
- `url` — string
- `exists` — boolean
- `page_id` — integer
- `categories[]` — string array
- `edit_count` — integer
- `first_created` — date
- `last_edited` — date
- `flagged_disputes` — boolean (talk page disputes, neutrality flags)

The edit history is analytically valuable: it reveals who is actively shaping the subject's public narrative. PR firms, political opponents, and subjects themselves frequently edit Wikipedia pages. Track `flagged_disputes` as a reputation signal.

---

### 7.2 Political party membership

```
type:        object[]
sensitivity: special_category
source:      electoral commission filings, party disclosures, news records
legal:       GDPR Art.9 — political opinion is explicitly listed
```

Each entry:
- `party_name` — string
- `country` — string
- `role` — string | null (member / officer / candidate / donor)
- `date_joined` — date | null
- `date_left` — date | null (null = current)

Distinct from inferred political leaning (macro-category 5.2) — this is declared, formal membership. Store separately and never merge with inferred scores.

---

### 7.3 Government positions held

```
type:        object[]
sensitivity: standard (but triggers PEP status)
source:      government directories, electoral records, official gazettes, news archives
```

Each entry:
- `title` — string
- `body` — string (e.g. "US Senate", "Ministry of Finance")
- `country` — string
- `level` — enum (federal / state / municipal / supranational)
- `role_type` — enum (elected / appointed / advisory)
- `date_from` — date
- `date_to` — date | null
- `is_current` — boolean
- `pep_trigger` — boolean (auto-set true for any entry)

Any government position — past or present — triggers PEP (Politically Exposed Person) classification for AML purposes. PEP status must persist for a defined period after leaving office (typically 1–5 years depending on jurisdiction).

---

### 7.4 Military service

```
type:        object | null
sensitivity: sensitive
source:      service records (via FOIA), veteran databases, official biographies, news
```

Subfields:
- `country` — string
- `branch` — string
- `rank_final` — string
- `rank_at_discharge` — string
- `units[]` — string array
- `deployments[]` — object array (location, date_from, date_to)
- `discharge_type` — enum (honorable / general / other_than_honorable / dishonorable / medical)
- `clearance_level` — enum (confidential / secret / top_secret / sci / unknown) | null
- `date_enlisted` — date
- `date_discharged` — date | null

Discharge type is a major character signal. Security clearance level implies historical access to classified material — significant for counterintelligence and background check contexts.

---

### 7.5 Lobbying relationships

```
type:        object[]
sensitivity: standard
source:      FARA filings (US), EU Transparency Register, UK lobbying register, national equivalents
```

Each entry:
- `registrant_name` — string (lobbying firm or self)
- `client` — string
- `legislative_body` — string
- `issues[]` — string array (topics lobbied on)
- `fees_usd` — float | null
- `date_from` — date
- `date_to` — date | null
- `filing_id` — string
- `source_url` — string

Reveals hidden financial relationships between individuals and organizations. A person may appear publicly neutral while receiving significant fees for advocacy. Cross-reference with government positions held to identify revolving-door patterns.

---

## Macro-category 8: Health & vulnerability

> Medical status, dependencies, and sensitive conditions.  
> All fields are GDPR Art.9 special category and/or HIPAA-regulated.  
> Require the strictest access controls, audit logging, and legal basis documentation.

### 8.1 Hospitalization history

```
type:        object[]
sensitivity: special_category
source:      insurance claims, medical record breaches, disability filings, court records
legal:       HIPAA (US), GDPR Art.9 (EU) — strict basis required
```

Each entry:
- `facility` — string
- `admission_date` — date
- `discharge_date` — date | null
- `reason_category` — enum (surgical / psychiatric / emergency / chronic / unknown)
- `source` — string
- `source_type` — enum (public_record / insurance / court_document / self_disclosed)

Do not store diagnosis details unless sourced from a public legal record and legally permissible. Reason category is sufficient for most intelligence purposes.

---

### 8.2 Insurance provider

```
type:        object[]
sensitivity: sensitive
source:      HR records, public benefit filings, court documents, breach data
```

Each entry:
- `type` — enum (health / life / disability / long_term_care)
- `provider` — string
- `policy_number` — string | null
- `employer_sponsored` — boolean
- `date_from` — date
- `date_to` — date | null

Indirectly reveals employer relationship, financial status, and (via provider type) health condition categories. Disability insurance in particular is a signal for underlying health conditions.

---

### 8.3 Mental health history

```
type:        object[]
sensitivity: special_category
source:      public court records (limited), disability claims, security clearance adjudications
legal:       HIPAA, GDPR Art.9 — highest restriction tier
access:      restrict to need-to-know; full audit log required
```

Each entry:
- `condition_category` — enum (mood / anxiety / psychotic / personality / substance / other)
- `treatment_type` — enum (inpatient / outpatient / medication / unknown)
- `date_approx` — date (year-level precision is sufficient)
- `source` — string
- `is_public_record` — boolean

This is the highest-sensitivity field in the schema. Linked to stigma, discrimination, security clearance revocation, and custody disputes. Only store what appears in a verifiable public record. Never infer or speculate. Gate access at the application layer — not all users of the platform should see this field.

---

### 8.4 Disabilities

```
type:        object[]
sensitivity: special_category
source:      disability benefit filings, ADA accommodation records, self-disclosed, court documents
legal:       GDPR Art.9, ADA (US), Equality Act (UK) — restricted use
```

Each entry:
- `category` — enum (physical / cognitive / sensory / psychiatric / other)
- `description` — string | null
- `source` — string
- `is_public_record` — boolean

Anti-discrimination law restricts how disability data can be used in employment, insurance, and service contexts. Do not use as a scoring factor. Store only when sourced from a public legal record and document the legal basis for processing.

---

## Cross-cutting implementation notes

### Sensitivity tiers

| Tier | Label | Examples | Access |
|------|-------|----------|--------|
| 1 | `standard` | Wikipedia page, work address, military service | Default |
| 2 | `sensitive` | Home address, criminal record, IP history | Role-gated |
| 3 | `special_category` | Biometrics, mental health, political opinion, ethnicity | Need-to-know + audit log |

### Fields that must be live lookups (never static cache)

- OFAC SDN list
- EU consolidated sanctions list
- INTERPOL red notices
- PEP status (derived from government positions — recalculate on update)

### Fields that are inferred vs. declared

Always store `inference_method` and `confidence` alongside any inferred field. Never merge inferred data with declared data into a single value. Key inferred fields:

- Political leaning (inferred from behavior vs. declared via party registration)
- Personality type (inferred via NLP vs. self-reported)
- Crypto wallet attribution (clustering vs. self-disclosed)
- Physical description details (inferred from media vs. from official ID)

### Graph edges

The following fields are not just data points — they are edges in a relationship graph. Each should resolve to a linked entity record where possible:

- `emergency_contacts[].linked_person_id`
- `lobbying_relationships[].client` → organization entity
- `military_service[].units[]` → unit entity
- `government_positions[].body` → institution entity

### Regulatory frameworks to review before deployment

- **GDPR** — EU/EEA: Art.9 special categories, Art.22 automated decision-making
- **CCPA / CPRA** — California: sensitive personal information definition
- **HIPAA** — US: health data
- **BIPA** — Illinois: biometric data
- **FARA** — US: lobbying disclosures
- **Ban-the-Box laws** — US (varies by state): arrest history in employment
- **FCRA** — US: consumer reporting, background checks
