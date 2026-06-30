/** Create/update (by code) an external-org-kind catalog entry. name is the default-locale fallback. */
export interface IUpsertExternalOrgKindRequest {
    'code': string;
    'name': string;
    'sortOrder'?: number | null;
}
