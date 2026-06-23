/** A board / senate / council / committee of an institution. */
export interface IGovernanceBody {
    'id': string;
    'institutionId': string;
    'code': string;
    'name': string;
    'kind': string;
    'mandate'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
