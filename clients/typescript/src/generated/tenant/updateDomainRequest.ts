/** Rename / retire a domain. Omitted fields are unchanged. `code` is immutable. */
export interface IUpdateDomainRequest {
    'name'?: string | null;
    'status'?: string | null;
    'sortOrder'?: number | null;
}
