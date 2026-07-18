/**
 * One permission code held by a MACHINE subject (M51 / D-ServiceIdentities) — the reified
 * PRINCIPAL_GRANT link. FLAT by construction: no target unit, no scope, no graph, because a
 * service principal has no unit reach.
 *
 * `orgId` absent means INSTANCE-WIDE (reference-catalog imports, the M53 wiring codes); a named
 * organization confines the principal to that organization's data — the blast-radius boundary
 * for a connector is the organization, not the unit.
 *
 */
export interface IPrincipalGrant {
    'id': string;
    /** The service principal's URN RID (registered on IdentityFederationService). */
    'principalId': string;
    /** A code from the closed permission catalog. */
    'permission': string;
    /** Absent = instance-wide; present = confined to that organization. */
    'orgId'?: string | null;
    'grantedBy'?: string | null;
    'grantedAt': string;
    'revokedAt'?: string | null;
}
