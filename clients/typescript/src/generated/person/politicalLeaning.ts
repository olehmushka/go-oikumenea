/**
 * An INFERRED political leaning (D-PersonOverlays, M35) — an Object. Political opinion is a GDPR
 * Art. 9 special category, so `spectrum` is envelope-encrypted at rest; the value here is decrypted
 * (0 for a crypto-erased tombstone). legalBasis is required. This is a SEPARATE overlay and is NEVER
 * merged with the declared M33 party membership. pii:special.
 *
 */
export interface IPoliticalLeaning {
    'id': string;
    'personId': string;
    /** The inferred left/right position in [-1,1]. Decrypted; 0 when crypto-erased. */
    'spectrum': number | "NaN";
    /** The inference methodology tags, e.g. social_media | voting_record | donation_records. */
    'inferenceSources': Array<string>;
    'assessedAt'?: string | null;
    /** The platform_legal_basis_kinds code authorizing this Art. 9 inference. */
    'legalBasis': string;
    'confidence'?: string | null;
}
