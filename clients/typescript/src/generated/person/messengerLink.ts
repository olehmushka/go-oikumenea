/**
 * Reachability on a messenger platform over one of the person's existing channels
 * (D-PersonSocialChannels). Exactly one of phoneId/emailId is set.
 *
 */
export interface IMessengerLink {
    'id': string;
    /** The URN RID of the person's phone this link annotates; null when the link is over an email. */
    'phoneId'?: string | null;
    /** The URN RID of the person's email this link annotates; null when the link is over a phone. */
    'emailId'?: string | null;
    /** The messenger platform code (category=messenger). */
    'platformCode': string;
    /** The person's primary messenger reachability (at most one active). */
    'isPrimary': boolean;
    /** When the reachability was verified; null when unverified. */
    'verifiedAt'?: string | null;
}
