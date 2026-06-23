/** A person holds a grant (PI / co-investigator / researcher). */
export interface IGrantHolding {
    'id': string;
    'personId': string;
    'grantId': string;
    'role': string;
    'status': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
