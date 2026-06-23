/** A person was awarded a scholarship. */
export interface IScholarshipAward {
    'id': string;
    'personId': string;
    'scholarshipId': string;
    'status': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
