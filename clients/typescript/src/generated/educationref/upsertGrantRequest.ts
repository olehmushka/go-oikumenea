export interface IUpsertGrantRequest {
    'code': string;
    'title': string;
    'funder'?: string | null;
    'funderRef'?: string | null;
    'amount'?: string | null;
    'currency'?: string | null;
    'startOn'?: string | null;
    'endOn'?: string | null;
    'status'?: string | null;
}
