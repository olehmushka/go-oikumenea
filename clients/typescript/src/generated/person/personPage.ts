import { IPerson } from "./person";

/** A page of persons plus the opaque token for the next page (empty when exhausted). */
export interface IPersonPage {
    'persons': Array<IPerson>;
    'nextPageToken'?: string | null;
}
