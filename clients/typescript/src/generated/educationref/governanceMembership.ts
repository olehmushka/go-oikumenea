/** A person is a member of a governance body. */
export interface IGovernanceMembership {
    'id': string;
    'personId': string;
    'bodyId': string;
    'roleInBody'?: string | null;
    'status': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
