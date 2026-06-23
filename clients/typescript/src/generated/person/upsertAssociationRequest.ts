/** Record or replace a symmetric association between the path person and counterpartId. */
export interface IUpsertAssociationRequest {
    'id'?: string | null;
    /** The other person's URN RID. */
    'counterpartId': string;
    /** associate | coi | no_contact. */
    'kind': string;
    /** Optional relation-type code (category=association). */
    'relationCode'?: string | null;
    /** active | ended; defaults to active. */
    'status'?: string | null;
}
