import { IRole } from "./role";

/** A page of roles plus the opaque token for the next page (empty when exhausted). */
export interface IRolePage {
    'roles': Array<IRole>;
    'nextPageToken'?: string | null;
}
