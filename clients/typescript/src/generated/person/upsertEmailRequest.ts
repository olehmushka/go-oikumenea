/** Add a contact email, or replace one when id is supplied. provider is derived from the address. */
export interface IUpsertEmailRequest {
    /** The URN RID of an existing email row to replace; omit to add a new row. */
    'id'?: string | null;
    'typeCode': string;
    'address': string;
    'isPrimary'?: boolean | null;
}
