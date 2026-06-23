/** A per-tradition place/site type (church/cathedral/mosque/synagogue/temple/…). */
export interface ISiteType {
    'id': string;
    'traditionTaxonId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
