/** Add a personality profile, or replace one when id is supplied (D-PersonOverlays, M35). */
export interface IUpsertPersonalityRequest {
    'id'?: string | null;
    'framework'?: string | null;
    'result': string;
    'instrument'?: string | null;
    'method'?: string | null;
    'assessedAt'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
