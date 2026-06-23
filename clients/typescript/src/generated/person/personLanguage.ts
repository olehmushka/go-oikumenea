/**
 * A language a person speaks (D-Languages, M18; Link link__speaks). languageId references a
 * level='language' Glottolog languoid (LanguageService); name is the languoid's translatable
 * display name (locale -> text map). cefrLevel is the optional CEFR proficiency; isNative flags a
 * mother tongue.
 *
 */
export interface IPersonLanguage {
    'id': string;
    'personId': string;
    /** The languoid's URN RID (a level='language' node; resolve via GET /language/v1/languages). */
    'languageId': string;
    /** The languoid's translatable display name as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** CEFR proficiency — one of A1 | A2 | B1 | B2 | C1 | C2; null when unstated. */
    'cefrLevel'?: string | null;
    /** Whether this is a mother tongue. */
    'isNative': boolean;
}
