/** A country in the ISO-3166-1 registry. `id` is the RID (the reference key); `code` is the stable ISO-3166-1 alpha-2 lookup code; `name` is the default-locale (English) name. */
export interface ICountry {
    /** The country's RID (location service); what person/document/rank reference. */
    'id': string;
    /** ISO-3166-1 alpha-2 code (e.g. UA, PL); the stable, locale-agnostic lookup key. */
    'code': string;
    /** Default-locale (English) display name; other locales arrive via the i18n store. */
    'name': string;
    /** active | retired. */
    'status': string;
}
