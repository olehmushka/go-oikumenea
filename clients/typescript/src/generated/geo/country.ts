/** A country in the ISO-3166-1 registry. `id` is the RID (the reference key); `code` is the stable ISO-3166-1 alpha-2 lookup code; `name` is the locale->text display name. */
export interface ICountry {
    /** The country's RID (location service); what person/document/rank reference. */
    'id': string;
    /** ISO-3166-1 alpha-2 code (e.g. UA, PL); the stable, locale-agnostic lookup key. */
    'code': string;
    /** locale->text display name (all enabled locales; default-locale `name` column + i18n store, keyed by country code). */
    'name': { [key: string]: string };
    /** active | retired. */
    'status': string;
}
