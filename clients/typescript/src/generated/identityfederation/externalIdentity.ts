/**
 * A verified (issuer, subject) login point federating to one account (Object External
 * identity). Globally unique; immutable once created and removed by unlink. No token columns —
 * access/refresh tokens are never persisted.
 *
 */
export interface IExternalIdentity {
    /** The external identity's URN RID (carried as a plain string). */
    'id': string;
    /** The URN RID of the account this identity federates to. */
    'accountId': string;
    /** The IdP `iss` (an allowed issuer). */
    'issuer': string;
    /** The IdP `sub` — a pseudonymous identifier for the person. */
    'subject': string;
    'createdAt': string;
}
