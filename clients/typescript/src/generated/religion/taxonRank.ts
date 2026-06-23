/** An ordered structural level in the taxonomy scaffold (religion → branch → … → denomination). */
export interface ITaxonRank {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'ordinal': number;
    'status': string;
    'sortOrder'?: number | null;
}
