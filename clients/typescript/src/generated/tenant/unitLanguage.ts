/**
 * A unit's official/working language (D-Languages, M18; Link link__unit_language). languageId
 * references a Glottolog languoid (LanguageService); name is its translatable display name.
 *
 */
export interface IUnitLanguage {
    'id': string;
    'unitId': string;
    /** The languoid's URN RID (resolve via GET /language/v1/languages). */
    'languageId': string;
    /** The languoid's translatable display name as a locale -> text map (all enabled locales; D-i18n). */
    'name': { [key: string]: string };
    /** Whether the language is official (vs. merely working) for the unit. */
    'isOfficial': boolean;
}
