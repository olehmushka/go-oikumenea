/** Link a verified (issuer, subject) login point to an account. */
export interface ILinkIdentityRequest {
    'issuer': string;
    'subject': string;
}
