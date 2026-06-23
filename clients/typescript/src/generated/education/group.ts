/** A cohort (study group) under a unit, with an admission year. */
export interface IGroup {
    'id': string;
    'unitId': string;
    'code': string;
    'name': { [key: string]: string };
    'admissionYear'?: number | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
