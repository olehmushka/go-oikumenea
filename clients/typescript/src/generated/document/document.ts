/** A person-held paper of some type — metadata only (no binaries). number/issuer are pii:basic. */
export interface IDocument {
    /** The document's URN RID. */
    'id': string;
    /** The holder's person URN RID. */
    'personId': string;
    /** The document type's URN RID. */
    'typeId': string;
    /** The document number (passport no., licence no.). pii:basic. */
    'number'?: string | null;
    /** Issuing authority (e.g. ДМС України). pii:basic. */
    'issuer'?: string | null;
    /** Country RID of the issuing country (resolve via GET /geo/countries); lets one person hold same-type papers from several countries. */
    'issuingCountry'?: string | null;
    /** ISO-8601 date the document was issued. */
    'issuedOn'?: string | null;
    /** ISO-8601 date the document expires. */
    'expiresOn'?: string | null;
    /** Free-form long-tail per-type fields (JSONB). pii:special ceiling. */
    'attributes'?: any | null;
    /** One of active | superseded | revoked — self-asserted, reversible, orthogonal to deletion. */
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
