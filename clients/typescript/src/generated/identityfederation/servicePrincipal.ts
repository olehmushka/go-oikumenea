/**
 * A MACHINE subject (M51 / D-ServiceIdentities) — a facade with standing of its own, or a
 * connector. It authenticates through the same external IdP and the same middleware as a
 * person, using the OAuth2 client-credentials grant, and is keyed by the same
 * (issuer, subject) pair as an ExternalIdentity: a given pair is a person identity XOR a
 * principal, never both.
 *
 * A principal holds NO role assignment and NO unit reach; its authority is the flat
 * per-principal grants owned by AuthorizationService. Registering one creates no credential —
 * the IdP owns the client secret (L-AuthzOnly holds).
 *
 */
export interface IServicePrincipal {
    /** The principal's URN RID (carried as a plain string). */
    'id': string;
    /** Stable, locale-agnostic machine name (D-Code) — what audit rows and operators reference. */
    'code': string;
    'name': string;
    'description'?: string | null;
    /** The IdP `iss` of the client-credentials token. */
    'issuer': string;
    /** The IdP `sub` of the client-credentials token. Immutable after registration. */
    'subject': string;
    /**
     * Display label from the token's `azp`/`client_id` claim, so an operator can tell which IdP
     * client this is. NEVER an authorization input — the identity key is (issuer, subject).
     *
     */
    'clientId'?: string | null;
    /** One of active | disabled. A disabled principal fails resolution, so its tokens stop working at once. */
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
