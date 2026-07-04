/**
 * A person's political-party affiliation (D-InstitutionalTies, M33) — a reified link. Political
 * opinion is a GDPR Art. 9 special category, so the party identity is envelope-encrypted at rest;
 * `party` here is the decrypted value ("" for a crypto-erased tombstone). legalBasis is required.
 * pii:special.
 *
 */
export interface IPartyMembership {
    'id': string;
    'personId': string;
    /** The party name (or an external-organizations RID). Decrypted; "" when crypto-erased. */
    'party': string;
    /** One of member | official | candidate | donor | supporter | other. */
    'role': string;
    'validFrom'?: string | null;
    /** ISO-8601 date the affiliation ended; null = current. */
    'validTo'?: string | null;
    /** The platform_legal_basis_kinds code authorizing this Art. 9 processing. */
    'legalBasis': string;
    /** active | retired. */
    'status': string;
    'source'?: string | null;
    'confidence'?: string | null;
}
