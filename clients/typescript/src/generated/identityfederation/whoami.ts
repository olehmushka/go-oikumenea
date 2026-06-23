/**
 * The caller's own resolved PDP context — the person (and optional account) the inbound token
 * mapped to. Produced from the request context the validation middleware attached.
 *
 */
export interface IWhoami {
    /** The URN RID of the resolved person (the PDP subject). */
    'personId': string;
    /** The URN RID of the resolved account; absent only on out-of-band/system contexts. */
    'accountId'?: string | null;
    /** The account's IdP-asserted email, if any. */
    'email'?: string | null;
}
