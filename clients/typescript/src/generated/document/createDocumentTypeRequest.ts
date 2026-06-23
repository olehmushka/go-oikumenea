/** Add a document (paper) type to the catalog. */
export interface ICreateDocumentTypeRequest {
    'code': string;
    /** Default-locale label; translatable via the localization store. */
    'name': string;
    /** Optional per-type attribute schema (D-DocumentAttrSchema); validated on document write when set. */
    'attrSchema'?: any | null;
    'sortOrder'?: number | null;
}
