/** An in-directory next-of-kin nomination (subject→contact, both directory persons; D-PersonRelationships; Link link__next_of_kin). A nomination, not a blood fact. */
export interface INextOfKin {
    'id': string;
    /** The nominating person's URN RID. */
    'subjectId': string;
    /** The nominated contact's URN RID (an in-directory person). */
    'contactId': string;
    /** Optional relation-type catalog code (category=next_of_kin). */
    'relationCode'?: string | null;
    /** Priority ordering of the nomination (1 = highest). */
    'priority': number;
    /** One of active | withdrawn. */
    'status': string;
}
