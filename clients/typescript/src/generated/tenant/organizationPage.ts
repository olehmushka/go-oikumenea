import { IOrganization } from "./organization";

/** A page of organizations plus the opaque token for the next page (empty when exhausted). */
export interface IOrganizationPage {
    'organizations': Array<IOrganization>;
    'nextPageToken'?: string | null;
}
