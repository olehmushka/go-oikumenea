/**
 * A person's insurance coverage (D-HealthVulnerability, M36) — an Object. pii:sensitive; gated on
 * person.read. Hard-erased on purge.
 *
 */
export interface IInsurance {
    'id': string;
    'personId': string;
    /** One of health | life | disability | ltc. */
    'type': string;
    'provider'?: string | null;
    'policyReference'?: string | null;
    /** True when the coverage is employer-sponsored. */
    'employerSponsored': boolean;
    'validFrom'?: string | null;
    'validTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
