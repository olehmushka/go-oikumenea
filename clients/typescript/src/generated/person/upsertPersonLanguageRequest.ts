/** Add or update a language the person speaks (keyed on languageId). cefrLevel/isNative are the proficiency attributes. */
export interface IUpsertPersonLanguageRequest {
    /** The languoid's URN RID; must resolve to a level='language' Glottolog node. */
    'languageId': string;
    /** A1 | A2 | B1 | B2 | C1 | C2; omit to leave unstated. */
    'cefrLevel'?: string | null;
    'isNative'?: boolean | null;
}
