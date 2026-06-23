/** Enable/disable, rename, set default, or reorder a locale (instance-admin; `locale.manage`). Omitted fields are unchanged. */
export interface IUpdateLocaleRequest {
    'name'?: string | null;
    'enabled'?: boolean | null;
    /** Setting true makes this the sole default; the previous default is cleared in the same transaction. */
    'isDefault'?: boolean | null;
    'sortOrder'?: number | null;
}
