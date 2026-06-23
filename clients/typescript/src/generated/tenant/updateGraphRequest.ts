/** Rename / set default / flip isAuthorityBearing (guarded). Omitted fields are unchanged. */
export interface IUpdateGraphRequest {
    'name'?: string | null;
    /** Setting true makes this the sole default; the previous default is cleared in the same transaction. */
    'isDefault'?: boolean | null;
    'isAuthorityBearing'?: boolean | null;
}
