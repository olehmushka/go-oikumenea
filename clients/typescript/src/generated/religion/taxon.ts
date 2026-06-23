/** A node in the recursive faith taxonomy. parentId is null for a root religion. */
export interface ITaxon {
    'id': string;
    'parentId'?: string | null;
    'rankId': string;
    /** The level marker code (religion/branch/tradition/sub_tradition/denomination). */
    'rankCode': string;
    /** The denormalized root religion taxon (derived via the closure). */
    'religionId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    'description'?: string | null;
    'wikidataId'?: string | null;
    'sortOrder'?: number | null;
    /** Distance from the queried/root ancestor (populated by list/search via the closure). */
    'depth'?: number | null;
    'createdAt': string;
    'updatedAt': string;
}
