/** Update number/issuer/issuing-country/validity/attributes/status. Omitted fields are unchanged. */
export interface IUpdateDocumentRequest {
    'number'?: string | null;
    'issuer'?: string | null;
    'issuingCountry'?: string | null;
    'issuedOn'?: string | null;
    'expiresOn'?: string | null;
    'attributes'?: any | null;
    /** New status — active | superseded | revoked (reversible). */
    'status'?: string | null;
}
