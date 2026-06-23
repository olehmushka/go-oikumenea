export interface IAddOrgClassificationRequest {
    'taxonId': string;
    'isPrimary'?: boolean | null;
    'source'?: string | null;
    'confidence'?: string | null;
}
