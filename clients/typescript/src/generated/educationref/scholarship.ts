/** A financial award scheme (institution or external). */
export interface IScholarship {
    'id': string;
    'institutionId'?: string | null;
    'code': string;
    'name': string;
    'kind': string;
    'amount'?: string | null;
    'currency'?: string | null;
    'frequency': string;
    'renewable': boolean;
    'conditions'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
