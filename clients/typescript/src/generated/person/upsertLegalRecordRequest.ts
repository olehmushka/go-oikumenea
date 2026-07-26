/**
 * Add a legal record, or replace one when id is supplied (D-LegalRecords, M38). Requires
 * `disposition` (arrest ≠ guilt) and legalBasis (Art. 10). Category-level only — never a full
 * charge sheet.
 *
 */
export interface IUpsertLegalRecordRequest {
    'id'?: string | null;
    'kind': string;
    'disposition': string;
    'detail': string;
    'jurisdiction'?: string | null;
    'occurredAt'?: string | null;
    'dispositionDate'?: string | null;
    'isSuppressed'?: boolean | null;
    'suppressedReason'?: string | null;
    'legalBasis': string;
    'source'?: string | null;
    'confidence'?: string | null;
}
