/** Add or update a unit's official/working language (keyed on languageId). */
export interface IUpsertUnitLanguageRequest {
    /** The languoid's URN RID. */
    'languageId': string;
    /** Defaults to true (official); false marks a working language. */
    'isOfficial'?: boolean | null;
}
