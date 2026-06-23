/** A degree/diploma/certificate program of an institution (optionally a unit). */
export interface IProgram {
    'id': string;
    'institutionId': string;
    'owningUnitId'?: string | null;
    'degreeLevelId'?: string | null;
    'code': string;
    'name': string;
    'mode': string;
    'durationYears'?: string | null;
    'creditHoursTotal'?: number | null;
    'state': string;
    'createdAt': string;
    'updatedAt': string;
}
