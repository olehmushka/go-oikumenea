import { IExternalIdentity } from "./externalIdentity";

/**
 * An optional login attachment to exactly one person (Object Account). A person may have zero
 * accounts (roster-only) or one active account. `identities` are the linked login points and is
 * populated by getAccount/createAccount; it is empty in contexts that do not load it.
 *
 */
export interface IAccount {
    /** The account's URN RID (carried as a plain string). */
    'id': string;
    /** The URN RID of the person this account attaches to. */
    'personId': string;
    /** The IdP-asserted email, if any; unique among active accounts when present. */
    'email'?: string | null;
    /** One of active | disabled (disable is a reversible login block). */
    'status': string;
    /** The external identities (login points) federated to this account. */
    'identities': Array<IExternalIdentity>;
    'createdAt': string;
    'updatedAt': string;
}
