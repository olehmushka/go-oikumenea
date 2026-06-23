export interface IUpsertClergyGradeRequest {
    'traditionTaxonId'?: string | null;
    'gradeCategoryId': string;
    'code': string;
    'name': string;
    'ordinal': number;
    'sortOrder'?: number | null;
}
