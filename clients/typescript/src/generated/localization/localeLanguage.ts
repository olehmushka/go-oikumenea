/**
 * A supported locale's canonical Glottolog language (D-Languages, M18; Link link__locale_language).
 * Read-only: the link is reconciled by the language-scheme import (matching the locale's ISO-639-3
 * code to a languoid's iso639_3), not edited directly.
 *
 */
export interface ILocaleLanguage {
    /** The locale's ISO 639-3 code. */
    'locale': string;
    /** The matched languoid's URN RID. */
    'languageId': string;
    /** The languoid's translatable display name as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
}
