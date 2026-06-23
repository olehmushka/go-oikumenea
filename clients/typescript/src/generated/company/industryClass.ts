/** An economic-activity classification entry (NACE/ISIC/KVED). */
export interface IIndustryClass {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    /** One of nace | isic | kved. */
    'system': string;
    'status': string;
    'sortOrder'?: number | null;
}
