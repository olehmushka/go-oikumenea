/**
 * A public office a person holds or held (D-InstitutionalTies, M33) — a reified link. pepTrigger
 * persists after the position ends and feeds the M34 PEP watchlist check. pii:basic.
 *
 */
export interface IGovernmentPosition {
    'id': string;
    'personId': string;
    /** e.g. "Minister of Defence", "Senator". */
    'title': string;
    /** The government body, e.g. "Ministry of Defence", "US Senate". */
    'body': string;
    /** Optional resolved body RID (polymorphic — external org / company / unit). */
    'orgId'?: string | null;
    'countryId'?: string | null;
    /** One of international | national | regional | local. */
    'level': string;
    'roleType'?: string | null;
    'validFrom'?: string | null;
    'validTo'?: string | null;
    /** Politically-exposed-person flag; persists after the office ends. */
    'pepTrigger': boolean;
    'source'?: string | null;
    'confidence'?: string | null;
}
