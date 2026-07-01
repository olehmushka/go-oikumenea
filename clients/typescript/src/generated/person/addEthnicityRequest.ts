/** Declare an ethnicity for a person (self-declared, envelope-encrypted, lawful-basis-gated). */
export interface IAddEthnicityRequest {
    /** A person_ethnicity_types code (validated against the catalog before sealing). */
    'code': string;
    /** The GDPR lawful-basis code (an Art. 9 condition; platform_legal_basis_kinds). */
    'legalBasis': string;
    'source'?: string | null;
    'confidence'?: string | null;
}
