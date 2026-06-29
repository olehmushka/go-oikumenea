/** Rename / retire a unit kind or adjust its attr schema. Omitted fields are unchanged. `code`/`domainId` are immutable. */
export interface IUpdateUnitKindRequest {
    'name'?: string | null;
    'attrSchema'?: any | null;
    'status'?: string | null;
    'sortOrder'?: number | null;
}
