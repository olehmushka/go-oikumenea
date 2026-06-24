/** An administrative place in the WOF geo_places gazetteer (D-GeoPlaces). `id` is the RID; used e.g. as a vehicle plate region (placetype=region). */
export interface IPlace {
    /** The place's RID (location service); what consumers (e.g. vehicle registrations) reference. */
    'id': string;
    /** country | region | county | locality. */
    'placetype': string;
    /** Default-locale display name; other locales arrive via the i18n store. */
    'name': string;
    /** The RID of the containing country. */
    'countryId'?: string | null;
    /** active | retired. */
    'status': string;
}
