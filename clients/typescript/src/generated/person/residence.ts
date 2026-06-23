/** A person's effective-dated residence in a country/region (D-Geo). Locator data (pii:contact). */
export interface IResidence {
    'id': string;
    'personId': string;
    /** Country RID (resolve via GET /geo/countries). */
    'country': string;
    /** Optional sub-national region / locality (free text). */
    'region'?: string | null;
    /** ISO-8601 date the residence began. */
    'validFrom': string;
    /** ISO-8601 date the residence ended; null = current. */
    'validTo'?: string | null;
}
