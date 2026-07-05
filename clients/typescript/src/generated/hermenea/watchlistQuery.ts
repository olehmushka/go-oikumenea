/**
 * A person-identity query for a live watchlist screening check (D-Watchlists, M34). oikumenea sends
 * this; hermenea owns egress to the OFAC/EU/UN/INTERPOL providers + a ≤24h cache. Only match
 * metadata comes back — never the lists themselves.
 *
 */
export interface IWatchlistQuery {
    /** An opaque caller key (the person RID) used to key the ≤24h cache; not sent upstream. */
    'subjectKey': string;
    /** The name screened against the providers. */
    'fullName': string;
    /** ISO-8601 date, when known, to disambiguate matches. */
    'birthdate'?: string | null;
    /** ISO-3166 alpha-2, when known. */
    'nationality'?: string | null;
}
