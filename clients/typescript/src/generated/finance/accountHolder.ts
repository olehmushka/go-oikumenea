/**
 * The ownership edge (link__held_by). The holder is a person OR a company (polymorphic). Temporal:
 * an ended holding closes effectiveTo, so the edge set is the holding history.
 *
 */
export interface IAccountHolder {
    'id': string;
    'accountId': string;
    /** One of person | company. */
    'holderKind': string;
    'holderId': string;
    /** Best-effort display label (company legal name for company holders; empty for persons). */
    'holderLabel'?: string | null;
    /** One of primary | joint | authorized_signer. */
    'role': string;
    'effectiveFrom': string;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
