/** Add or replace the active citizenship for a country (keyed by (person, country)). */
export interface IUpsertCitizenshipRequest {
    'country': string;
    /** birth | descent | naturalization | other; defaults to other. */
    'basis'?: string | null;
    'acquiredOn'?: string | null;
    'lostOn'?: string | null;
    'isPrimary'?: boolean | null;
}
