/** A sponsor→sponsored link — godparent / academic advisor / military mentor (D-PersonRelationships; Link link__sponsor_of). */
export interface ISponsorship {
    'id': string;
    /** The sponsor's URN RID. */
    'sponsorId': string;
    /** The sponsored person's URN RID. */
    'sponsoredId': string;
    /** Required relation-type catalog code (category=sponsorship). */
    'relationCode': string;
    /** One of active | ended. */
    'status': string;
    'effectiveFrom'?: string | null;
    /** null = ongoing. */
    'effectiveTo'?: string | null;
    /** Optional education context — the enrollment (D-Education, M20) this sponsorship relates to. */
    'enrollmentId'?: string | null;
    /** Optional education role of the sponsor — one of professor | tutor | curator | advisor. */
    'educationRole'?: string | null;
}
