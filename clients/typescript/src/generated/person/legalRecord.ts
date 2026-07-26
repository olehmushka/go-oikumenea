/**
 * A category-level criminal / arrest / court record (D-LegalRecords, M38) — an Object. Criminal
 * data is GDPR Art. 10, so the category-level offence `detail` is envelope-encrypted at rest; the
 * value here is decrypted ("" for a crypto-erased tombstone). CATEGORY-LEVEL ONLY — never a full
 * charge sheet. `disposition` is mandatory (arrest ≠ guilt). legalBasis is required and reads need
 * the person.legal-record.read code. Sealed/expunged records (isSuppressed) are withheld unless the
 * caller also holds person.legal-record.read-suppressed. pii:special.
 *
 */
export interface ILegalRecord {
    'id': string;
    'personId': string;
    /** One of criminal_conviction | arrest | court_judgment. */
    'kind': string;
    /** Mandatory outcome — one of convicted | acquitted | dismissed | pending | sealed | expunged | no_charges. */
    'disposition': string;
    /** The decrypted category-level offence/charge (coarse, NO full charge sheet); "" when crypto-erased. */
    'detail': string;
    /** ISO-3166-1 country code of the issuing jurisdiction (D-Geo). */
    'jurisdiction'?: string | null;
    /** ISO date of the offence / arrest. */
    'occurredAt'?: string | null;
    /** ISO date the disposition was reached. */
    'dispositionDate'?: string | null;
    /** True when the record is sealed or expunged (withheld from the normal read gate). */
    'isSuppressed': boolean;
    /** One of sealed | expunged — present iff isSuppressed. */
    'suppressedReason'?: string | null;
    /** The platform_legal_basis_kinds code authorizing this Art. 10 record. */
    'legalBasis': string;
    'source'?: string | null;
    'confidence'?: string | null;
}
