/** A unit of study / module / subject of an institution (optionally a unit). */
export interface ICourse {
    'id': string;
    'institutionId': string;
    'owningUnitId'?: string | null;
    'code': string;
    'title': string;
    'creditHours'?: number | null;
    'level'?: number | null;
    'description'?: string | null;
    'deliveryMode': string;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
