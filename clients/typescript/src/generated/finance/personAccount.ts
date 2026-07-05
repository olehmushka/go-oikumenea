/** An account a person holds, enriched with the bank + the person's holding role (read-only). */
export interface IPersonAccount {
    'id': string;
    'institutionId': string;
    'institutionLabel'?: string | null;
    'currency'?: string | null;
    'accountTypeLabel'?: string | null;
    /** The person's holding role on this account (primary | joint | authorized_signer). */
    'role': string;
    'status': string;
    'createdAt': string;
}
