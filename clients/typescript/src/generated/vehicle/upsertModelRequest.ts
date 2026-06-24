export interface IUpsertModelRequest {
    'code': string;
    'name': string;
    'generation'?: string | null;
    'manufactureStart'?: string | null;
    'manufactureEnd'?: string | null;
    'sortOrder'?: number | null;
}
