/**
 * A person's self-declared ethnicity (D-PhysicalIdentity / D-SpecialPII, M31). The declared value
 * is envelope-encrypted at rest; `code` is the DECRYPTED catalog code returned only to authorized
 * readers. A crypto-erased tombstone returns an empty code.
 *
 */
export interface IEthnicity {
    'id': string;
    'personId': string;
    /** The declared ethnicity-type code (decrypted); empty for a crypto-erased tombstone. */
    'code': string;
    /** The catalog's default-locale display name for the declared code. */
    'name'?: string | null;
    /** The GDPR lawful-basis code (platform_legal_basis_kinds; an Art. 9 condition for special-category data). */
    'legalBasis': string;
    /** One of active | retired. */
    'status': string;
    'source'?: string | null;
    'confidence'?: string | null;
}
