/**
 * A public discovery hit: a site with its coordinate coarsened per publicPrecision (latitude/
 * longitude are null when the precision is `hidden`).
 *
 */
export interface IDiscoverySite {
    'id': string;
    'orgUnitId': string;
    'siteTypeId': string;
    'siteTypeCode': string;
    'siteTypeName': { [key: string]: string };
    'publicPrecision': string;
    'isPrimary': boolean;
    'latitude'?: number | "NaN" | null;
    'longitude'?: number | "NaN" | null;
}
