/** An ISCED 2011 degree level (0..8), a migration-seeded reference scale. */
export interface IDegreeLevel {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'iscedLevel': number;
    'status': string;
    'sortOrder'?: number | null;
}
