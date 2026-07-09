/**
 * A declared or formally-assessed personality profile (D-PersonOverlays, M35) — an Object.
 * DECLARED SURVEY OR FORMAL HR ASSESSMENT ONLY — never inferred from text. pii:sensitive.
 *
 */
export interface IPersonality {
    'id': string;
    'personId': string;
    /** One of mbti | big_five | disc | enneagram | other. */
    'framework': string;
    /** The typed output, e.g. "INTJ", a Big-Five summary, an Enneagram type. */
    'result': string;
    /** The specific test/assessment used. */
    'instrument'?: string | null;
    /** One of self_declared_survey | hr_assessment (inference is forbidden). */
    'method': string;
    'assessedAt'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
