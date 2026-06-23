/** Attach a paper to a person. The type and (optional) issuing country are validated against their registries. */
export interface ICreateDocumentRequest {
    'typeId': string;
    'number'?: string | null;
    'issuer'?: string | null;
    'issuingCountry'?: string | null;
    'issuedOn'?: string | null;
    'expiresOn'?: string | null;
    'attributes'?: any | null;
}
