/** Add or update a declared-ethnicity catalog entry (instance-admin managed). */
export interface IUpsertEthnicityTypeRequest {
    'code': string;
    /** The default-locale display name (other locales arrive via the localization store). */
    'name': string;
    /** The RID of the parent group (absent for a root). D-PhysicalIdentity amendment, M43. */
    'parentId'?: string | null;
    'wikidataId'?: string | null;
    'sortOrder'?: number | null;
}
