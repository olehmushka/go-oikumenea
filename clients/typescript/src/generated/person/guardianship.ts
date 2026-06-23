/** A legal guardian→ward link, distinct from blood kinship (D-PersonRelationships; Link link__guardian_of). */
export interface IGuardianship {
    'id': string;
    /** The guardian's URN RID. */
    'guardianId': string;
    /** The ward's URN RID. */
    'wardId': string;
    /** Optional relation-type catalog code. */
    'relationCode'?: string | null;
    /** One of active | ended. */
    'status': string;
    'effectiveFrom'?: string | null;
    /** null = ongoing. */
    'effectiveTo'?: string | null;
}
