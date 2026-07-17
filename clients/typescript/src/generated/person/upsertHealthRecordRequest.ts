/**
 * Add or replace the person's category-level health record for a kind (one active row per kind,
 * replaced in place). Requires legalBasis (Art. 9). Category-level only — never a diagnosis
 * (D-HealthVulnerability, M36).
 *
 */
export interface IUpsertHealthRecordRequest {
    'kind': string;
    'detail': string;
    'isPublicRecord'?: boolean | null;
    'assessedAt'?: string | null;
    'legalBasis': string;
    'source'?: string | null;
    'confidence'?: string | null;
}
