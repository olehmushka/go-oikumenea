/** A symmetric association / conflict-of-interest / prohibited-contact link (D-PersonRelationships; Link link__associated_with). */
export interface IAssociation {
    'id': string;
    /** The lower-sorting person's URN RID (canonical pair ordering). */
    'personIdA': string;
    /** The higher-sorting person's URN RID. */
    'personIdB': string;
    /** Optional relation-type catalog code (category=association). */
    'relationCode'?: string | null;
    /** One of associate | coi | no_contact. */
    'kind': string;
    /** One of active | ended. */
    'status': string;
}
