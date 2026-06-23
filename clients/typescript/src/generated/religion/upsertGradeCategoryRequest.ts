export interface IUpsertGradeCategoryRequest {
    'traditionTaxonId'?: string | null;
    'code': string;
    'name': string;
    'ordinal'?: number | null;
    'sortOrder'?: number | null;
}
