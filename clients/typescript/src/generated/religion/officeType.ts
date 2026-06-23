/** A clergy office-type label (offices themselves are filled as membership Positions). */
export interface IOfficeType {
    'id': string;
    'traditionTaxonId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    'status': string;
    'sortOrder'?: number | null;
}
