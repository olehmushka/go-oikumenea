# Plan: fix /locations country prefill + inline "create location" modal

## Context

Two web-console (`web/`) issues on the M19/M32 location + person-address surface:

1. **Bug — country prefills on partial coordinate input.** On `/locations`, typing only
   **Широта (latitude)** and blurring already fills the Country field. Root cause found in
   [locations/page.tsx:85-103](web/src/app/(dashboard)/locations/page.tsx#L85-L103):
   the guard is `if (!Number.isFinite(lat) || !Number.isFinite(lng)) return;`, but the empty
   longitude reads as `""` from FormData and **`Number("")` is `0`, not `NaN`** — `0` is finite,
   so the guard passes and `resolveCoordinate(lat, 0)` reverse-geocodes a bogus point and prefills
   a wrong country. Prefill must fire only when the coordinate is actually complete.

2. **No inline location creation.** A person's addresses already have a UI
   ([AddressManager in PersonForms.tsx:608-692](web/src/app/(dashboard)/persons/[personId]/PersonForms.tsx#L608-L692))
   that picks a location via [`SearchSelect kind="location"`](web/src/components/SearchSelect.tsx).
   But the picker can only find **pre-existing** locations — to add an address to a not-yet-recorded
   place, the operator must leave, go to `/locations`, create it, come back, and search. The user
   wants a **"＋ create location" modal** available from every location picker; on create, the new
   location is auto-selected in the form.

## Approach

### Part 1 — fix the prefill guard (small, central)

In the extracted location form (see Part 2), replace the ineffective numeric guard with an
empty-string check on the raw values **before** converting:

```ts
const latRaw = String(f.get("latitude") || "").trim();
const lngRaw = String(f.get("longitude") || "").trim();
if (latRaw === "" || lngRaw === "") return;          // both fields required, not just finite
const lat = Number(latRaw);
const lng = Number(lngRaw);
if (!Number.isFinite(lat) || !Number.isFinite(lng)) return;
```

Only the `latlon` mode does client-side prefill; the other modes (MGRS/UTM/СК-42/СК-42 grid) derive
country server-side on submit and never prefill, so no change is needed there. This one fix makes
prefill fire only when **both** lat and lon are present.

### Part 2 — extract a reusable `LocationForm` (removes duplication, hosts the fix)

Create **`web/src/components/LocationForm.tsx`** by lifting the form body of `CreateLocation`
(the format selector, `CoordinateFields`, `buildCoordinate`, country/type/locality/street inputs,
the fixed `onLatLonBlur`, and the submit → `api.location.createLocation`) out of
[locations/page.tsx](web/src/app/(dashboard)/locations/page.tsx). It:

- self-fetches location types (`api.location.listLocationTypes()`) so it is drop-in anywhere,
- takes `onCreated: (loc: Location) => void` and an optional `submitLabel`,
- resets its inputs + controlled prefill state after a successful create (existing behaviour).

Move `CoordinateFields` and `buildCoordinate` into this file too.

Then **refactor `locations/page.tsx`**: `CreateLocation` becomes a thin wrapper rendering
`<LocationForm onCreated={setCreated} />` plus the existing green success card. Net effect: the page
keeps working, the bug fix lives in exactly one place, and the same form is reusable in a modal.

### Part 3 — a simple `Modal` primitive

No dialog library exists (no radix/shadcn). Add **`web/src/components/Modal.tsx`** following the
codebase's `useState`-toggle convention: a fixed full-screen overlay + centered `Card`, closed on
backdrop click / Esc / a close button. **Render via `createPortal` to `document.body`** so the
modal's inner `<form>` (from `LocationForm`) is NOT nested inside the outer address `<form>` — nested
forms are invalid HTML and would break submit. API roughly:
`<Modal open title onClose>{children}</Modal>`.

### Part 4 — "＋ create" affordance on the location picker (covers every place)

Augment **[`SearchSelect`](web/src/components/SearchSelect.tsx)** with an opt-in `allowCreate` prop.
When set and `kind === "location"`, render a small "＋" button beside the search input that opens a
`Modal` containing `<LocationForm onCreated={...}>`. On create, build the picker `Result`
(`{ id: loc.id, label: loc.locality || loc.mgrs || coords, hint }` — reuse the existing
`REGISTRY.location.toResult` shape) and call the internal `choose(...)` so the new location is
immediately selected and the hidden `<input name>` is populated. Close the modal.

Enable it in **AddressManager** ([PersonForms.tsx:675](web/src/app/(dashboard)/persons/[personId]/PersonForms.tsx#L675)):
`<SearchSelect kind="location" name="locationId" required allowCreate placeholder="Search a location…" />`.
Because the affordance lives in the shared `SearchSelect`, any current/future location picker gets it
by adding the one prop — satisfying "in each place where I can choose location".

## Files

- **new** `web/src/components/Modal.tsx` — portal-based modal primitive.
- **new** `web/src/components/LocationForm.tsx` — reusable create-location form (holds the prefill fix, `CoordinateFields`, `buildCoordinate`).
- **edit** `web/src/app/(dashboard)/locations/page.tsx` — `CreateLocation` wraps `LocationForm`; remove the now-moved form/coordinate code.
- **edit** `web/src/components/SearchSelect.tsx` — add `allowCreate`, the "＋" button, and the create modal wiring.
- **edit** `web/src/app/(dashboard)/persons/[personId]/PersonForms.tsx` — pass `allowCreate` to the address picker.

No backend, API, migration, or SDK changes — `api.location.createLocation`, `listLocationTypes`,
`geo.resolveCoordinate`, and the person address endpoints already exist.

## Verification

1. `cd web && npm run lint && npx tsc --noEmit` (type-check the refactor).
2. `cd web && npm run build` (Next build passes).
3. Manual (via `/run` or dev server):
   - **Bug 1:** `/locations`, choose WGS84 lat/lon, type only Широта and blur → **Country stays empty**. Fill both lat+lon and blur → Country prefills. Confirm MGRS/UTM/СК-42 modes are unaffected.
   - **Bug 2:** open a person → Addresses card → click "＋" on the location picker → modal opens with the full create-location form → create a location → modal closes and the new location is **auto-selected** in the picker → submit the address and confirm it lists.
