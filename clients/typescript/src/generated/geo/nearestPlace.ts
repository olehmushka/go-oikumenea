/** The gazetteer place nearest a coordinate (D-GeoPlaces) — a city/town/village (placetype=locality) when one exists, else the nearest county/region. Powers the locations-form prefill. */
export interface INearestPlace {
    /** The place's RID (location service). */
    'id': string;
    /** locality | county | region. */
    'placetype': string;
    /** Default-locale display name; other locales arrive via the i18n store. */
    'name': string;
    /** The RID of the containing country. */
    'countryId'?: string | null;
    /** Great-circle distance in metres from the input coordinate to the place's centroid. */
    'distanceMeters': number | "NaN";
}
