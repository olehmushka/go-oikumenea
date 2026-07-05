/**
 * The persisted residue of a live watchlist screening check (D-Watchlists, M34). Match METADATA
 * only — never the underlying lists. One row per person; CheckWatchlists refreshes it. `pep` is a
 * snapshot of the M33 government-position derivation captured at check time. pii:sensitive.
 *
 */
export interface IWatchlistMatch {
    'id': string;
    'personId': string;
    /** Any hit across the queried providers (OFAC / EU / UN / INTERPOL). */
    'onList': boolean;
    /** The list codes matched, e.g. INTERPOL_RED, OFAC_SDN. */
    'lists': Array<string>;
    'program'?: string | null;
    /** 0..1 best-match score across providers. */
    'matchScore'?: number | "NaN" | null;
    /** Politically-exposed-person snapshot, derived from M33 government positions. */
    'pep': boolean;
    'lastChecked': string;
    /** RFC-3339 time the upstream ≤24h cache lapses. */
    'nextCheckDue'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
