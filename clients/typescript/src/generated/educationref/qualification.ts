/** A formally awardable qualification (degree) classification of an institution. */
export interface IQualification {
    'id': string;
    'institutionId': string;
    'programId'?: string | null;
    'degreeLevelId'?: string | null;
    'code': string;
    'name': string;
    'frameworkCode'?: string | null;
    'frameworkLevel'?: string | null;
    'awardingBody'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
