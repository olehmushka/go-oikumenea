/** Create/update (by code) a legal-form catalog entry. name is the default-locale fallback. */
export interface IUpsertLegalFormRequest {
    'code': string;
    'name': string;
    'abbreviation'?: string | null;
    'countryId'?: string | null;
    'sortOrder'?: number | null;
}
