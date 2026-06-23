/** Begin reversible deactivation; opens a grace window (purgeAfter = now + the configured grace). */
export interface IDeactivateRequest {
    /** Optional free-text reason recorded in the audit entry. */
    'reason'?: string | null;
}
