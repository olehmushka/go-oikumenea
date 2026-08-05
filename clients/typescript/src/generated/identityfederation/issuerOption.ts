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
    /**
     * The expected `aud`, if the instance pins one. When the issuer accepts several (one per
     * client of this deployment) this carries the first; `audiences` carries the full set.
     *
     */
    'audience'?: string | null;
    /**
     * Every `aud` this instance accepts from the issuer. A token validates when its own
     * audience intersects this set. Non-empty for every `oidc` issuer (enforced at boot).
     *
     */
    'audiences': Array<string>;
    /**
     * Operator-supplied display name for the issuer ("Google", "Corporate Entra ID"), so a
     * binding UI can offer a readable choice instead of a bare discovery URL. Cosmetic only —
     * never an identity or authorization input.
     *
     */
    'label'?: string | null;
    /** One of oidc | hs256 (hs256 is local/dev only). */
    'type': string;
}
