/**
 * One deduped login/IP occurrence for an account (M37 / D-LoginSecurityLog): first-party
 * security telemetry from the token-validation seam, `pii:contact`, retention-bounded. Repeat
 * occurrences from the same (context, ip) within the dedup window collapse into one row —
 * `occurrenceCount` + `lastSeenAt` carry the recency. The `resolved*` / `isVpn` / `isTor`
 * fields are the IP-intelligence overlay and stay absent until a resolver ships.
 *
 */
export interface ILoginEvent {
    'id': string;
    'accountId': string;
    /** One of login | activity | registration (registration = the JIT link-on-match). */
    'context': string;
    'ip': string;
    'firstSeenAt': string;
    'lastSeenAt': string;
    'occurrenceCount': number;
    /** ISO 3166-1 alpha-2 when resolved. */
    'resolvedCountry'?: string | null;
    'resolvedIsp'?: string | null;
    'isVpn'?: boolean | null;
    'isTor'?: boolean | null;
    'userAgent'?: string | null;
}
