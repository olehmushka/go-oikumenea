/** A tradition tag on a unit (link__classified_as), one of which may be primary. */
export interface IOrgClassification {
    'id': string;
    'unitId': string;
    'taxonId': string;
    'taxonCode': string;
    'taxonName': { [key: string]: string };
    'isPrimary': boolean;
    'source'?: string | null;
    'confidence'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
