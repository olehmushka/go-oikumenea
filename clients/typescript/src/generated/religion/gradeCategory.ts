/** A per-tradition grouping of clergy grades (e.g. Christianity → major/minor orders). */
export interface IGradeCategory {
    'id': string;
    /** The tradition taxon this category is scoped to; null = generic across faiths. */
    'traditionTaxonId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    'ordinal'?: number | null;
    'status': string;
    'sortOrder'?: number | null;
}
