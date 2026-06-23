/** Create/update an institution-kind or unit-kind. name is the default-locale fallback. */
export interface IUpsertCatalogKindRequest {
    'code': string;
    'name': string;
    'sortOrder'?: number | null;
}
