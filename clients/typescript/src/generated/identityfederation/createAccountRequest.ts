import { ILinkIdentityRequest } from "./linkIdentityRequest";

/**
 * Create an account for a person, optionally linking its first external identity in the same
 * operation. The first identity on a new account is always permitted (the linking cap applies
 * only to ADDITIONAL identities).
 *
 */
export interface ICreateAccountRequest {
    'personId': string;
    /** The IdP-asserted email, if known. */
    'email'?: string | null;
    /** The first login point to link; omit to create a login-less account shell. */
    'identity'?: ILinkIdentityRequest | null;
}
