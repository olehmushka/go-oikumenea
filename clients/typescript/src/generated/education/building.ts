/** A physical building of an institution (optionally a unit), located via the shared M19 location. */
export interface IBuilding {
    'id': string;
    'institutionId': string;
    'unitId'?: string | null;
    /** The shared location RID (M19); null until geocoded. */
    'locationId'?: string | null;
    'code': string;
    'name': { [key: string]: string };
    /** One of academic | dormitory | administrative | library | sports | other. */
    'kind': string;
    'createdAt': string;
    'updatedAt': string;
}
