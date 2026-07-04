/** Add a party membership, or replace one when id is supplied (D-InstitutionalTies, M33). */
export interface IUpsertPartyMembershipRequest {
    'id'?: string | null;
    'party': string;
    'role'?: string | null;
    'validFrom'?: string | null;
    'validTo'?: string | null;
    'legalBasis': string;
    'status'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
