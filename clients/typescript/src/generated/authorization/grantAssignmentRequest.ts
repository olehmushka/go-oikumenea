/** Grant a role to a person at a target unit. graph applies to subtree scope (default command); ignored for unit. */
export interface IGrantAssignmentRequest {
    'subjectPersonId': string;
    'roleId': string;
    'targetUnitId': string;
    /** One of unit | subtree. */
    'scope': string;
    /** The cascade graph CODE for a subtree grant; omit to default to the command graph. Ignored for unit scope. */
    'graph'?: string | null;
    /** Optional decision-time expiry (RFC 3339). */
    'expiresAt'?: string | null;
}
