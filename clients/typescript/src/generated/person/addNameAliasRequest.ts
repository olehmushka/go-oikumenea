/** Add an alias name form (aka/former_legal/maiden/pseudonym/cover). Addressed by its RID; may carry attribution. */
export interface IAddNameAliasRequest {
    'locale': string;
    /** One of aka | former_legal | maiden | pseudonym | cover. */
    'variantKind': string;
    'displayName': string;
    'title'?: string | null;
    'given'?: string | null;
    'given2'?: string | null;
    'surname'?: string | null;
    'surnamePrefix'?: string | null;
    'surname2'?: string | null;
    'generation'?: string | null;
    'credentials'?: string | null;
    'preferred'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
