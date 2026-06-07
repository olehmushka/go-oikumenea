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

/** Pick the best label for `locale`, falling back through eng/ukr, then any present key. */
export function pickLabel(map: LocaleMap, locale: string = DEFAULT_LOCALE): string {
  if (!map) return "";
  if (map[locale]) return map[locale];
  for (const fb of UI_LOCALES) if (map[fb]) return map[fb];
  const first = Object.values(map)[0];
  return first ?? "";
}
