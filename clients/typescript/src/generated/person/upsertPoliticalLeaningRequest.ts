/**
 * Set the person's inferred political leaning (replaces the single active row). Requires legalBasis
 * (Art. 9). Inferred-only — never a declared party affiliation (D-PersonOverlays, M35).
 *
 */
export interface IUpsertPoliticalLeaningRequest {
    /** The inferred position in [-1,1]. */
    'spectrum': number | "NaN";
    'inferenceSources'?: Array<string> | null;
    'assessedAt'?: string | null;
    'legalBasis': string;
    'confidence'?: string | null;
}
