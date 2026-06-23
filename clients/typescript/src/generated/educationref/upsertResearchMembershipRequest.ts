export interface IUpsertResearchMembershipRequest {
    'groupId': string;
    'role'?: string | null;
    'status'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
