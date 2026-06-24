export interface IUpsertBrandRequest {
    'code': string;
    'name': string;
    'countryId'?: string | null;
    'sortOrder'?: number | null;
}
