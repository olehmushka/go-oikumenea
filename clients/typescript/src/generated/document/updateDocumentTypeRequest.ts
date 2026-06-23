/** Edit a document type. `code` is immutable by convention. */
export interface IUpdateDocumentTypeRequest {
    'name'?: string | null;
    /** Replace the per-type attribute schema (D-DocumentAttrSchema). Omitted leaves it unchanged. */
    'attrSchema'?: any | null;
    'status'?: string | null;
    'sortOrder'?: number | null;
}
