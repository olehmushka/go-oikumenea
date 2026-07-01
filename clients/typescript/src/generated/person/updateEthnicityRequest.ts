/** Re-declare the ethnicity value and/or flip legal basis / status. */
export interface IUpdateEthnicityRequest {
    'code': string;
    'legalBasis': string;
    /** active | retired; defaults to active. */
    'status'?: string | null;
}
