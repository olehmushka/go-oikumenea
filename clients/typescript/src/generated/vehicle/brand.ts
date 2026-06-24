/** A vehicle marque (Toyota/BMW…); countryId is the country of origin (nullable). */
export interface IBrand {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'countryId'?: string | null;
    'status': string;
    'sortOrder'?: number | null;
}
