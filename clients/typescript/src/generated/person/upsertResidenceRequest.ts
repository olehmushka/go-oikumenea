/** Add a residence row, or replace one when id is supplied. */
export interface IUpsertResidenceRequest {
    /** The URN RID of an existing residence row to replace; omit to add a new row. */
    'id'?: string | null;
    'country': string;
    'region'?: string | null;
    'validFrom': string;
    'validTo'?: string | null;
}
