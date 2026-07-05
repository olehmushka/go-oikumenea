/**
 * A bank account at registry grade. The RID is the external handle (no separate code). The IBAN
 * is envelope-encrypted at rest; `iban` is populated (decrypted) only on getAccount for authorized
 * callers, never in a list. Soft-deleted, not destroyed.
 *
 */
export interface IAccount {
    'id': string;
    /** The holding bank — a `company`-domain tenant_organizations RID. */
    'institutionId': string;
    /** Best-effort bank organization name. */
    'institutionLabel'?: string | null;
    /** The decrypted IBAN; present only on getAccount for authorized callers. */
    'iban'?: string | null;
    /** ISO 4217 (e.g. UAH, USD). */
    'currency'?: string | null;
    'accountTypeId'?: string | null;
    'accountTypeLabel'?: string | null;
    /** One of active | closed | frozen. */
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
