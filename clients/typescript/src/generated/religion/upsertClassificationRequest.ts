export interface IUpsertClassificationRequest {
    'code': string;
    'name': string;
    'description'?: string | null;
    'sortOrder'?: number | null;
}
