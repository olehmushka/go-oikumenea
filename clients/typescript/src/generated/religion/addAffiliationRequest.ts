export interface IAddAffiliationRequest {
    'affiliationTypeId': string;
    'religionId'?: string | null;
    'traditionUnitId'?: string | null;
    'communityUnitId'?: string | null;
    /** An optional free-form belief detail; stored envelope-encrypted (pii:special). */
    'value'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
