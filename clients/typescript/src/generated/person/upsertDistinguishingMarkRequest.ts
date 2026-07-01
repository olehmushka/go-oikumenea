/** Add a distinguishing mark, or replace one when id is supplied. */
export interface IUpsertDistinguishingMarkRequest {
    /** The RID of an existing mark row to replace; omit to add a new row. */
    'id'?: string | null;
    /** One of tattoo | scar | piercing | birthmark. */
    'kind': string;
    'bodyLocation'?: string | null;
    'description'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
