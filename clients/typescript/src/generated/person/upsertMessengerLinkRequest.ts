/**
 * Add a messenger link over one of the person's phones or emails, or replace one when id is
 * supplied. Exactly one of phoneId/emailId must be set; platformCode must be a messenger platform.
 *
 */
export interface IUpsertMessengerLinkRequest {
    /** The URN RID of an existing messenger-link row to replace; omit to add a new row. */
    'id'?: string | null;
    'phoneId'?: string | null;
    'emailId'?: string | null;
    'platformCode': string;
    'isPrimary'?: boolean | null;
    'verifiedAt'?: string | null;
}
