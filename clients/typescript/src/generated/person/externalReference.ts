/**
 * An off-platform pointer about a person (D-InstitutionalTies, M33) — an Object. Mirrors a social
 * account; a hermenea import target (idempotent by URL). pii:basic.
 *
 */
export interface IExternalReference {
    'id': string;
    'personId': string;
    /** One of wikipedia | news | registry | social | court | academic | other. */
    'kind': string;
    'url': string;
    'externalId'?: string | null;
    'categories': Array<string>;
    'lastChecked'?: string | null;
    'disputed': boolean;
    'source'?: string | null;
    'confidence'?: string | null;
}
