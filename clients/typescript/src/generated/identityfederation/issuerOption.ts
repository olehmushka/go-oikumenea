/**
 * One IdP issuer accepted by this instance (from install config), offered to binding UIs so an
 * operator picks an issuer rather than typing it. PUBLIC fields only — verification secrets
 * (HS256 keys) are never exposed. An external identity's `issuer` must match one of these for a
 * token from it to ever validate.
 *
 */
export interface IIssuerOption {
    /** The `iss` value (also the OIDC discovery base URL). */
    'issuer': string;
    /** The expected `aud`, if the instance pins one. */
    'audience'?: string | null;
    /** One of oidc | hs256 (hs256 is local/dev only). */
    'type': string;
}
