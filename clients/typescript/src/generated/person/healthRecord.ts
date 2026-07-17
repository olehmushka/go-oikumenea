/**
 * A category-level health & vulnerability record (D-HealthVulnerability, M36) — an Object. Health is
 * a GDPR Art. 9 special category, so the category-level `detail` is envelope-encrypted at rest; the
 * value here is decrypted ("" for a crypto-erased tombstone). CATEGORY-LEVEL ONLY — never a diagnosis,
 * never inferred. legalBasis is required and reads need the person.health.read code. pii:special.
 *
 */
export interface IHealthRecord {
    'id': string;
    'personId': string;
    /** One of hospitalization | mental_health | disability. */
    'kind': string;
    /** The decrypted category-level note (coarse, NO diagnosis); "" when crypto-erased. */
    'detail': string;
    /** True when derived from a public record. */
    'isPublicRecord': boolean;
    'assessedAt'?: string | null;
    /** The platform_legal_basis_kinds code authorizing this Art. 9 record. */
    'legalBasis': string;
    'source'?: string | null;
    'confidence'?: string | null;
}
