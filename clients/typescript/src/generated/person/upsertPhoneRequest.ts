/** Add a contact phone, or replace one when id is supplied. number is E.164-normalized and country derived. */
export interface IUpsertPhoneRequest {
    /** The URN RID of an existing phone row to replace; omit to add a new row. */
    'id'?: string | null;
    'typeCode': string;
    'number': string;
    'isPrimary'?: boolean | null;
}
