/**
 * i18n helpers (D-i18n). The API returns translatable labels as a `locale → text` map in
 * every response — there is no Accept-Language negotiation. The UI picks a label for the
 * current UI locale here, with fallback, and writes the whole map back when editing.
 *
 * Locales are ISO 639-3 (seeded `ukr` + `eng`). Person names are NOT in these maps — they
 * use the per-person transliteration variants on the person record.
 */

export type LocaleMap = Record<string, string> | null | undefined;

export const UI_LOCALES = ["eng", "ukr"] as const;
export type UiLocale = (typeof UI_LOCALES)[number];
export const DEFAULT_LOCALE: UiLocale = "eng";

export const LOCALE_LABELS: Record<string, string> = {
  eng: "English",
  ukr: "Українська",
};

/**
 * Pick the best label for `locale`, falling back through eng/ukr, then any present key. When no
 * locale is passed, it resolves against the module-global active UI locale (see below) rather than a
 * fixed default — so the many `pickLabel(map)` call sites across bespoke pages honour the UI switch
 * without each having to thread the locale.
 */
export function pickLabel(map: LocaleMap, locale: string = getActiveLocale()): string {
  if (!map) return "";
  if (map[locale]) return map[locale];
  for (const fb of UI_LOCALES) if (map[fb]) return map[fb];
  const first = Object.values(map)[0];
  return first ?? "";
}

// ── active UI locale (module-global) ─────────────────────────────────────────
// The ontology registry's name accessors are pure, isomorphic, module-level functions with no access
// to React context, so they resolve `locale → text` maps against this module-global active locale
// instead of a hardcoded default. It is set per request on the SERVER (root layout, from the
// `ui-locale` cookie) and kept in sync on the CLIENT by LocaleProvider — which re-renders its
// consumers on switch, so the interactive views (DataTable, palette, drawer, graph) re-read it. Reads
// are synchronous within a single, await-free render pass, which keeps it coherent under Node's
// single-threaded server rendering.
let activeLocale: string = DEFAULT_LOCALE;

/** Set the active UI locale read by the registry label accessors (root layout + LocaleProvider). */
export function setActiveLocale(locale: string | null | undefined): void {
  activeLocale = locale || DEFAULT_LOCALE;
}

/** The active UI locale read by the registry label accessors. */
export function getActiveLocale(): string {
  return activeLocale;
}
