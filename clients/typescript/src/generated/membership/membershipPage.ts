import { IMembership } from "./membership";

/** A page of memberships plus the opaque token for the next page (empty when exhausted). */
export interface IMembershipPage {
    'memberships': Array<IMembership>;
    'nextPageToken'?: string | null;
}
