export interface IUpsertPolicyRequest {
    'code': string;
    'title': string;
    'governanceBodyId'?: string | null;
    'supersedesId'?: string | null;
    'kind'?: string | null;
    'effectiveOn'?: string | null;
    'expiryOn'?: string | null;
    'documentUrl'?: string | null;
    'status'?: string | null;
}
