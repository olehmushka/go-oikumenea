/** An ordered, per-tradition clergy grade. ordinal orders ONLY within a tradition (no cross-tradition comparator). */
export interface IClergyGrade {
    'id': string;
    'traditionTaxonId'?: string | null;
    'gradeCategoryId': string;
    'code': string;
    'name': { [key: string]: string };
    'ordinal': number;
    'status': string;
    'sortOrder'?: number | null;
}
