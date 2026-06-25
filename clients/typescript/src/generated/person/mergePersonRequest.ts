/** Resolve a provisional stub into a canonical person (D-OverlayFoundation, M29). */
export interface IMergePersonRequest {
    /** The URN RID of the surviving canonical person the stub's edges are re-homed onto. */
    'intoPersonId': string;
    /** Operator certainty about the identity equation — confirmed | probable | possible (defaults to possible). Recorded in the audit trail. */
    'confidence'?: string | null;
}
