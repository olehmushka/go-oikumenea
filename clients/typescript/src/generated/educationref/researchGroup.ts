/** A research cluster under a centre and/or unit of an institution. */
export interface IResearchGroup {
    'id': string;
    'institutionId': string;
    'centreId'?: string | null;
    'unitId'?: string | null;
    'code': string;
    'name': string;
    'focusArea'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
