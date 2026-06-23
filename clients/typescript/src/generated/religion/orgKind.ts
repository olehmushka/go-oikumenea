/** A descriptive organizational-level label for a religious-body unit (never branched on). */
export interface IOrgKind {
    'id': string;
    /** The religion taxon this kind is scoped to; null = generic across faiths. */
    'religionId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    'ordinal'?: number | null;
    'status': string;
    'sortOrder'?: number | null;
}
