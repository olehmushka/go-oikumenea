/** Flip status (active|lapsed|renounced) and/or re-encrypt a new belief value. */
export interface IUpdateAffiliationRequest {
    'status'?: string | null;
    'value'?: string | null;
}
