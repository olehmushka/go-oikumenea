/** Add a category under a system. `name` is the default-locale text; other locales via LocalizationService. */
export interface IAddCategoryRequest {
    /** The URN RID of the owning (active) rank system. */
    'systemId': string;
    'code': string;
    'name': string;
    /** Seniority ordinal; defaults to (max active sibling order + 1), i.e. appended last. */
    'sortOrder'?: number | null;
}
