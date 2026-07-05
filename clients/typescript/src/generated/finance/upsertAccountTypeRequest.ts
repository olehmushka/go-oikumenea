/** Create/update (by code) an account-type catalog entry. name is the default-locale fallback. */
export interface IUpsertAccountTypeRequest {
    'code': string;
    'name': string;
    'sortOrder'?: number | null;
}
