/** An institutional rule/regulation (optionally approved by a governance body). */
export interface IPolicy {
    'id': string;
    'institutionId': string;
    'governanceBodyId'?: string | null;
    'supersedesId'?: string | null;
    'code': string;
    'title': string;
    'kind': string;
    'effectiveOn'?: string | null;
    'expiryOn'?: string | null;
    'documentUrl'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
