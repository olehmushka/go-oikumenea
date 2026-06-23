export interface ICreateTaxonRequest {
    'code': string;
    'name': string;
    'rankId': string;
    /** The parent taxon; omit for a root religion. */
    'parentId'?: string | null;
    'description'?: string | null;
    'wikidataId'?: string | null;
    'sortOrder'?: number | null;
}
