/**
 * A worship-community unit's place: the reified link__site_of (Unit ↔ a shared Location,
 * D-Location). latitude/longitude carry the EXACT coordinate, returned to authorized owners; the
 * discovery search coarsens them per publicPrecision.
 *
 */
export interface ISite {
    'id': string;
    'orgUnitId': string;
    'locationId': string;
    'siteTypeId': string;
    'siteTypeCode': string;
    'siteTypeName': { [key: string]: string };
    /** public | unlisted | private. */
    'visibility': string;
    /** exact | street | neighborhood | city | hidden — the app-side publish-precision projection. */
    'publicPrecision': string;
    'isPrimary': boolean;
    'latitude': number | "NaN";
    'longitude': number | "NaN";
    'createdAt': string;
    'updatedAt': string;
}
