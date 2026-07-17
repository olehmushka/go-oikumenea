/** Add an insurance row, or replace one when id is supplied (D-HealthVulnerability, M36). */
export interface IUpsertInsuranceRequest {
    'id'?: string | null;
    'type': string;
    'provider'?: string | null;
    'policyReference'?: string | null;
    'employerSponsored'?: boolean | null;
    'validFrom'?: string | null;
    'validTo'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
