/**
 * A structured regulatory/enforcement action against a person (D-Watchlists, M34) — an Object.
 * Durable reference data (operator-curated or hermenea-imported), distinct from the volatile
 * live-lookup above. Idempotent import keys on (person, externalId). pii:sensitive.
 *
 */
export interface IRegulatorySanction {
    'id': string;
    'personId': string;
    /** The regulator/authority, e.g. "SEC", "FCA", "NBU". */
    'regulator': string;
    /** One of fine | ban | license_revocation | warning | settlement | debarment | other. */
    'actionType': string;
    'amount'?: number | "NaN" | null;
    /** ISO-4217 code; required when amount is present. */
    'currency'?: string | null;
    /** One of active | appealed | overturned | expired | settled. */
    'status': string;
    'sanctionDate'?: string | null;
    'sourceUrl'?: string | null;
    /** The id within the source system (the import idempotency key). */
    'externalId'?: string | null;
    'legalBasis'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
