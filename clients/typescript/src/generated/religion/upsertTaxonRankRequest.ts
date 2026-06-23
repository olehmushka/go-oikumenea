export interface IUpsertTaxonRankRequest {
    'code': string;
    'name': string;
    'ordinal': number;
    'sortOrder'?: number | null;
}
