/**
 * Record or replace a partnership between the path person and partnerId. The pair is stored in
 * canonical order; the path person must not be the partner. At most one active engaged/married
 * partnership per person.
 *
 */
export interface IUpsertPartnershipRequest {
    /** The URN RID of an existing partnership row to replace; omit to add a new row. */
    'id'?: string | null;
    /** The other partner's URN RID (an in-directory person). */
    'partnerId': string;
    /** engaged | married | divorced | widowed | annulled | dissolved. */
    'status': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
