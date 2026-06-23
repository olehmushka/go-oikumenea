/** Add a call sign, or replace one when id is supplied. The value is required and unique per person. */
export interface IUpsertCallSignRequest {
    /** The URN RID of an existing call-sign row to replace; omit to add a new row. */
    'id'?: string | null;
    'callSign': string;
    'isPrimary'?: boolean | null;
}
