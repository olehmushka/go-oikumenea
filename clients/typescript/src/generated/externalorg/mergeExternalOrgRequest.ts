/**
 * Resolve a provisional stub into a canonical organization. The provisional org (the path id) is
 * tombstoned; the canonical (intoOrgId) survives. confidence is recorded on the merge action.
 *
 */
export interface IMergeExternalOrgRequest {
    'intoOrgId': string;
    'confidence'?: string | null;
}
