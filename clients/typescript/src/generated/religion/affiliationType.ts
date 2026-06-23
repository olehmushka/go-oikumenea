/** A per-tradition lay-affiliation / milestone type (adherent/member; baptized; shahada; …). */
export interface IAffiliationType {
    'id': string;
    'traditionTaxonId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
