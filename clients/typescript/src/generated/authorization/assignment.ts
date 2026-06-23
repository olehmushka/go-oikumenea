/**
 * The reified link__has_role — the unit of granted authority. graphId is null for unit scope
 * and names the cascade graph for subtree scope. Reversible: revoking sets revokedAt rather
 * than deleting.
 *
 */
export interface IAssignment {
    /** The assignment's URN RID (carried as a plain string). */
    'id': string;
    'subjectPersonId': string;
    'roleId': string;
    'targetUnitId': string;
    /** One of unit | subtree. */
    'scope': string;
    /** The cascade graph RID (subtree only); null for unit scope. */
    'graphId'?: string | null;
    /** The granting person RID; null for the install bootstrap grant. */
    'grantedBy'?: string | null;
    'grantedAt': string;
    /** When revoked; null while active. */
    'revokedAt'?: string | null;
    'revokedBy'?: string | null;
    /** Optional decision-time expiry; null = no expiry. */
    'expiresAt'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
