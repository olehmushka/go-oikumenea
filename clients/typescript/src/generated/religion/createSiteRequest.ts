export interface ICreateSiteRequest {
    'locationId': string;
    'siteTypeId': string;
    /** public | unlisted | private (default public). */
    'visibility'?: string | null;
    /** exact | street | neighborhood | city | hidden (default exact). */
    'publicPrecision'?: string | null;
    'isPrimary'?: boolean | null;
}
