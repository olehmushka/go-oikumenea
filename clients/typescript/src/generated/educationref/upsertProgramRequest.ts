export interface IUpsertProgramRequest {
    'code': string;
    'name': string;
    'owningUnitId'?: string | null;
    'degreeLevelId'?: string | null;
    'mode'?: string | null;
    'durationYears'?: string | null;
    'creditHoursTotal'?: number | null;
    'state'?: string | null;
}
