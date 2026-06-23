/** An external reference institution (where people studied/taught). Soft-deleted, not destroyed. */
export interface IInstitution {
    'id': string;
    'code': string;
    'name': { [key: string]: string };
    'kindId': string;
    /** The institution's country RID (geo_countries); null for international/online. */
    'countryId'?: string | null;
    'foundedOn'?: string | null;
    'closedOn'?: string | null;
    /** One of active | closed | merged. */
    'state': string;
    'createdAt': string;
    'updatedAt': string;
}
