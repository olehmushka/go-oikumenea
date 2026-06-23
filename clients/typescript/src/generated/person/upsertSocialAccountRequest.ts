/** Add a social account, or replace one when id is supplied. handle is normalized and profileUrl derived when omitted. */
export interface IUpsertSocialAccountRequest {
    /** The URN RID of an existing social-account row to replace; omit to add a new row. */
    'id'?: string | null;
    'platformCode': string;
    'platformUserId'?: string | null;
    'handle': string;
    'displayName'?: string | null;
    'profileUrl'?: string | null;
    'language'?: string | null;
    'platformVerified'?: boolean | null;
    'verifiedByOperatorAt'?: string | null;
    /** self_declared | operator_verified | imported. */
    'source': string;
    /** confirmed | probable | possible; defaults to possible. */
    'confidence'?: string | null;
    'isPrimary'?: boolean | null;
}
