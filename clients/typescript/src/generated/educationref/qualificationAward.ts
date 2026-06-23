/** A person was awarded a qualification (the diploma award). */
export interface IQualificationAward {
    'id': string;
    'personId': string;
    'qualificationId': string;
    'enrollmentId'?: string | null;
    'awardedOn'?: string | null;
    'withDistinction': boolean;
    'gpa'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
