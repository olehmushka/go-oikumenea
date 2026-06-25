/**
 * Create a provisional stub person (D-OverlayFoundation, M29). Minimal-PII: only a display name
 * is required, with optional attribution (source/confidence) recording how the stub was learned.
 *
 */
export interface ICreateProvisionalPersonRequest {
    'displayName': string;
    /** How the stub was learned — self_declared | operator_verified | imported (the attribution convention). */
    'source'?: string | null;
    /** Certainty weight — confirmed | probable | possible (defaults to possible). */
    'confidence'?: string | null;
    'attributes'?: any | null;
}
