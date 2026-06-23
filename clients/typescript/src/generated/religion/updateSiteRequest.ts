/** Patch a site's type/visibility/precision/primary flag. Promoting to primary clears any existing primary. */
export interface IUpdateSiteRequest {
    'siteTypeId'?: string | null;
    'visibility'?: string | null;
    'publicPrecision'?: string | null;
    'isPrimary'?: boolean | null;
}
