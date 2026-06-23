/**
 * A person's standalone social-network account (D-PersonSocialChannels). platformUserId is the
 * platform's immutable internal id (the durable key); handle is the mutable current @handle.
 *
 */
export interface ISocialAccount {
    'id': string;
    'personId': string;
    /** The platform code the account is on. */
    'platformCode': string;
    /** The platform's immutable internal id (the durable key); null when unknown. */
    'platformUserId'?: string | null;
    /** The current @handle (mutable; rename history kept separately). */
    'handle': string;
    'displayName'?: string | null;
    /** Profile URL; derived from platform + handle on write when not supplied. */
    'profileUrl'?: string | null;
    'language'?: string | null;
    /** The platform "blue-check"; distinct from operator confirmation. */
    'platformVerified': boolean;
    /** When an operator confirmed the account; null when unconfirmed. */
    'verifiedByOperatorAt'?: string | null;
    /** How the account was learned — one of self_declared | operator_verified | imported. */
    'source': string;
    /** Weight of the claim — one of confirmed | probable | possible. */
    'confidence': string;
    /** The person's primary social account (at most one active). */
    'isPrimary': boolean;
}
