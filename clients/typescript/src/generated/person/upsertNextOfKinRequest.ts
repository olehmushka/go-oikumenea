/** Nominate or replace a next-of-kin contact for the path person (the subject). contactId must be an in-directory person. */
export interface IUpsertNextOfKinRequest {
    'id'?: string | null;
    /** The nominated contact's URN RID (an in-directory person). */
    'contactId': string;
    /** Optional relation-type code (category=next_of_kin). */
    'relationCode'?: string | null;
    /** Priority ordering (1 = highest); defaults to 1. */
    'priority'?: number | null;
    /** active | withdrawn; defaults to active. */
    'status'?: string | null;
}
