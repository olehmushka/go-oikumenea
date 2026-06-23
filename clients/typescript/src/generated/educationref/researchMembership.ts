/** A person is a member of a research group. */
export interface IResearchMembership {
    'id': string;
    'personId': string;
    'groupId': string;
    'role'?: string | null;
    'status': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
