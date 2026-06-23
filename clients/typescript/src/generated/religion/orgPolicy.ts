/** A data-driven eligibility/exclusion rule on a unit. */
export interface IOrgPolicy {
    'id': string;
    'unitId': string;
    'policyKindId': string;
    'policyKindCode': string;
    'reason'?: string | null;
    'decidedByPersonId'?: string | null;
    'decidedAt'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
