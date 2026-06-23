export interface IUpsertScholarshipRequest {
    'code': string;
    'name': string;
    'institutionId'?: string | null;
    'kind'?: string | null;
    'amount'?: string | null;
    'currency'?: string | null;
    'frequency'?: string | null;
    'renewable'?: boolean | null;
    'conditions'?: string | null;
    'status'?: string | null;
}
