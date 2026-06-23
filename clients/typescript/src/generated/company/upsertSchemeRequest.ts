/** Create/update (by code) a registration-scheme catalog entry. */
export interface IUpsertSchemeRequest {
    'code': string;
    'name': string;
    'validatorPattern'?: string | null;
    'isGlobal'?: boolean | null;
    'sortOrder'?: number | null;
}
