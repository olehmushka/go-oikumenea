# Fix slow person creation + auto-generate codes (frontend)

## Context

Two unrelated UX problems in the `web/` console, both fixed purely on the frontend.

**1. "Ultra-long" person creation.** The `POST /person/v1/persons` call is a fast single insert —
the perceived lag is in navigation. After create, `persons/new/page.tsx` calls
`router.push('/persons/{id}')`, and that detail page (`persons/[personId]/page.tsx`) is a server
component that **first** awaits the person, **then** awaits a `Promise.all` of **16 more** API calls
before it can render. The "Creating…" spinner stays up for that whole waterfall. Six of those calls
are system-wide catalogs (`platforms`, `relation-types`, `document-types`, `personal-code-schemes`,
`email-types`, `phone-types`) that are identical for every person and irrelevant to a brand-new empty
record. User chose **"Both"**: stream the shell *and* move catalogs client-side.

**2. No code autogeneration.** When the user leaves the `code` field empty on the person/position
create forms they must invent one. Desired: derive it from the name — transliterate Cyrillic→Latin,
lowercase, kebab-case, then append `-` + a nanoid suffix. User chose **frontend live-fill** (no
backend / Conjure-contract change): pre-fill the editable `code` input as the user types the name.

---

## Part A — Perf: stream the detail page + move catalogs client-side

### A1. Shared client catalog hook
New `web/src/lib/catalog.ts`: a `useCatalog<T>(path)` hook with a module-level cache keyed by path,
generalizing the existing pattern in `web/src/components/CountrySelect.tsx` (module `cache` var +
`bffGet` in `useEffect`). Returns `{ data, loading }`. This lets each manager widget fetch its own
catalog once, client-side, and reuse it across persons.

### A2. Managers self-fetch their catalogs — `persons/[personId]/PersonForms.tsx`
This file is already `"use client"` and already imports `bffGet`. Change these managers to call
`useCatalog(...)` instead of receiving the catalog as a prop:
- `EmailManager` → `/person/v1/person/email-types`
- `PhoneManager` → `/person/v1/person/phone-types`
- `SocialAccountManager` + `MessengerLinkManager` → `/person/v1/person/platforms`
- `RelationshipManager` → `/person/v1/person/relation-types`
- `DocumentManager` → `/document/v1/document-types`
- `PersonalCodeManager` → `/document/v1/personal-code-schemes`

Drop the now-unused `types` / `platforms` / `schemes` / `relationTypes` props from each component
signature.

### A3. Stream the page — `persons/[personId]/page.tsx`
- Remove the 6 catalog fetches from the server batch and stop passing those props.
- Fetch **only** `person` at the top (single round-trip) and render the header + Identity card +
  catalog-driven managers (contact / social / languages) immediately.
- Move the remaining **person-specific** fetches (documents, personal-codes, memberships, orders,
  partnerships, kinships, guardianships, sponsorships, next-of-kin, associations) into a child async
  server component (e.g. `PersonRelations`) and wrap it in `<Suspense fallback={…}>`. The page shell
  (and thus the end of the "Creating…" spinner) renders as soon as `person` resolves; the heavier
  sections stream in. Keep the existing per-call `.catch(() => …)` fallbacks.

Net effect: the create→navigate path goes from "1 sequential + 16 parallel awaits before any paint"
to "1 await → shell paints → rest streams", with the 6 shared catalogs no longer blocking and now
cached across the app.

---

## Part B — Frontend code autogeneration

### B1. Slug/transliteration util
New `web/src/lib/code.ts`:
- A Cyrillic→Latin map covering Ukrainian (KMU-2010 style: `і→i`, `и→y`, `г→h`, `ґ→g`, `х→kh`,
  `ц→ts`, `ч→ch`, `ш→sh`, `щ→shch`, `є→ie`, `ю→iu`, `я→ia`, `ь→''`, …) plus the extra Russian
  letters (`ё`, `ы`, `э`, `ъ`).
- `transliterate(s)` — lowercases, maps each rune through the table (pass-through for unmapped).
- `slugify(s)` — `transliterate` → NFKD normalize → `[^a-z0-9]+` to `-` → trim leading/trailing `-`.
- `generateCode(name)` — `slug ? `${slug}-${nano()}` : nano()`, where `nano = customAlphabet(...)`.

Add **`nanoid`** to `web/package.json` (`customAlphabet` over `0-9a-z`, length 6).

### B2. Wire live-fill into the create forms
Pattern (apply to both): controlled name + code inputs, a `codeTouched` flag, and a stable suffix in
a `useRef(nano())` (so the visible code doesn't churn per keystroke). The code input shows
`codeTouched ? code : (name ? `${slugify(name)}-${suffix}` : "")`; editing the code sets
`codeTouched`. Submit uses the effective value.
- `web/src/app/(dashboard)/persons/new/page.tsx` — derive from `displayName`. Code stays optional
  (send `undefined` if empty after trim).
- `web/src/components/PositionForms.tsx` (`CreatePosition`) — derive from `title`. The `code` input
  keeps `required`; pre-filling satisfies it.

No backend or Conjure changes — position `code` remains `required` in the contract and is always
populated by the pre-fill.

---

## Files

| File | Change |
| --- | --- |
| `web/src/lib/catalog.ts` | **new** — `useCatalog` cached hook |
| `web/src/lib/code.ts` | **new** — transliterate / slugify / generateCode |
| `web/package.json` | add `nanoid` dependency |
| `web/src/app/(dashboard)/persons/[personId]/page.tsx` | drop catalog fetches; fetch only person; Suspense-wrap streamed `PersonRelations` |
| `web/src/app/(dashboard)/persons/[personId]/PersonForms.tsx` | managers `useCatalog` instead of catalog props |
| `web/src/app/(dashboard)/persons/new/page.tsx` | live-fill code from displayName |
| `web/src/components/PositionForms.tsx` | live-fill code from title |

## Verification

1. `cd web && npm install` (pulls `nanoid`), then `npm run build` / `npx tsc --noEmit` — type-checks
   and builds clean (no leftover unused-prop references).
2. Run the stack (oikumenea + web). Create a person with a Cyrillic display name (e.g. `Іван
   Петренко`) and an empty code → the code input live-fills to `ivan-petrenko-xxxxxx`; on save the
   person is created with that code, and the redirect to the detail page paints the shell
   near-instantly (spinner no longer hangs), with documents/relationships/memberships streaming in.
3. Type a Latin name (e.g. `Commanding Officer`) in `CreatePosition` → code fills
   `commanding-officer-xxxxxx`; manually editing the code stops the auto-fill.
4. Open an existing person detail page and confirm the contact/social/document/relationship "add"
   widgets still populate their dropdowns (now fetched client-side via `useCatalog`) and that
   create/edit still works.
