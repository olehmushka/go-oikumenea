import { IAssignment } from "./assignment";

/** A page of assignments plus the opaque token for the next page (empty when exhausted). */
export interface IAssignmentPage {
    'assignments': Array<IAssignment>;
    'nextPageToken'?: string | null;
}
