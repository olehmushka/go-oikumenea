export interface ICreateAliasRequest {
    'aliasText': string;
    /** nickname | abbreviation | historical | misspelling | transliteration. */
    'aliasType': string;
    'locale'?: string | null;
}
