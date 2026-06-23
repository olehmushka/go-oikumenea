/** A search-only alternative name for an org unit (never displayed). */
export interface IAlias {
    'id': string;
    'unitId': string;
    'aliasText': string;
    /** nickname | abbreviation | historical | misspelling | transliteration. */
    'aliasType': string;
    'locale'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
