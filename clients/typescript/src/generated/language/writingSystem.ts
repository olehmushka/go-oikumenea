/** An ISO-15924 script. `id` is the RID; `code` is the ISO-15924 lookup code. */
export interface IWritingSystem {
    /** The writing system's RID (language service). */
    'id': string;
    /** ISO-15924 code (e.g. Latn, Cyrl, Hani). */
    'code': string;
    /** locale->text display name (all enabled locales; default-locale `name` column + i18n store). */
    'name': { [key: string]: string };
    /** logographic | syllabary | alphabet | abjad | abugida | featural. */
    'scriptType'?: string | null;
}
