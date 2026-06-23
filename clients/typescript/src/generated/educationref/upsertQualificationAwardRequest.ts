export interface IUpsertQualificationAwardRequest {
    'qualificationId': string;
    'enrollmentId'?: string | null;
    'awardedOn'?: string | null;
    'withDistinction'?: boolean | null;
    'gpa'?: string | null;
    'status'?: string | null;
}
