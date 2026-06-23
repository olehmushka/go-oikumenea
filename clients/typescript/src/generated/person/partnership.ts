/**
 * A marriage or engagement between two persons (D-PersonRelationships; Link link__partnered_with).
 * Symmetric: stored as a canonical pair (personIdA < personIdB). At most one active engaged/married
 * row per person.
 *
 */
export interface IPartnership {
    'id': string;
    /** The lower-sorting partner's URN RID (canonical pair ordering). */
    'personIdA': string;
    /** The higher-sorting partner's URN RID. */
    'personIdB': string;
    /** One of engaged | married | divorced | widowed | annulled | dissolved. */
    'status': string;
    /** ISO-8601 date the partnership began (YYYY-MM-DD). */
    'effectiveFrom'?: string | null;
    /** ISO-8601 date it ended (YYYY-MM-DD); null = ongoing. */
    'effectiveTo'?: string | null;
}
