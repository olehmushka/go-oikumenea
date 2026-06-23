/** Create/update (by code) an industry-class catalog entry. */
export interface IUpsertIndustryClassRequest {
    'code': string;
    'name': string;
    'system'?: string | null;
    'sortOrder'?: number | null;
}
