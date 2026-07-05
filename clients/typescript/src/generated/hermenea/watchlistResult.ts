/** Per-person match metadata returned by a screening check (D-Watchlists, M34). */
export interface IWatchlistResult {
    /** Any hit across the queried providers. */
    'onList': boolean;
    /** The list codes matched, e.g. INTERPOL_RED, OFAC_SDN. */
    'lists': Array<string>;
    'program'?: string | null;
    /** 0..1 best-match score across providers. */
    'matchScore'?: number | "NaN" | null;
    /** RFC-3339 time the check ran (cache hit or fresh). */
    'checkedAt': string;
    /** RFC-3339 time the cache lapses (checkedAt + TTL). */
    'nextCheckDue'?: string | null;
}
