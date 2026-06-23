/** Add a rank system (the top level). `name` is the default-locale text; other locales via LocalizationService. */
export interface IAddSystemRequest {
    'code': string;
    'name': string;
    /** Country RID of the national origin (resolve via GET /geo/countries); omit for a supranational system (NATO/UN). */
    'country'?: string | null;
    /** Order among active systems; defaults to appended last. */
    'sortOrder'?: number | null;
}
