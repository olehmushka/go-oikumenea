/** A legal entity at registry grade. Soft-deleted, not destroyed. */
export interface ICompany {
    'id': string;
    'code': string;
    'legalName': { [key: string]: string };
    'shortName'?: string | null;
    'legalFormId': string;
    /** One of private | public | state_owned | municipal | foreign | mixed. */
    'ownershipCategory': string;
    'countryId'?: string | null;
    'foundedOn'?: string | null;
    'dissolvedOn'?: string | null;
    /** One of active | dissolved | merged. */
    'state': string;
    'createdAt': string;
    'updatedAt': string;
}
