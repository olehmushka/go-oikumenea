export interface IAddClergyCredentialRequest {
    'clergyGradeId': string;
    'orgUnitId': string;
    /** The conferral date as a YYYY-MM-DD day string. */
    'grantedOn'?: string | null;
    'conferredByPersonId'?: string | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
