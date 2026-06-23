export interface IUpsertQualificationRequest {
    'code': string;
    'name': string;
    'programId'?: string | null;
    'degreeLevelId'?: string | null;
    'frameworkCode'?: string | null;
    'frameworkLevel'?: string | null;
    'awardingBody'?: string | null;
    'status'?: string | null;
}
