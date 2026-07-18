/**
 * Mutable display fields only. (issuer, subject) is the identity key the middleware resolves
 * on and is immutable — re-pointing it would silently transfer a machine's authority to a
 * different IdP client. Register a new principal and disable the old one instead.
 *
 */
export interface IUpdateServicePrincipalRequest {
    'name': string;
    'description'?: string | null;
    'clientId'?: string | null;
}
