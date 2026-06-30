import { IExternalOrganization } from "./externalOrganization";

export interface IExternalOrgPage {
    'orgs': Array<IExternalOrganization>;
    'nextPageToken'?: string | null;
}
