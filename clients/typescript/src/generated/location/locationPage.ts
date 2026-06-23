import { ILocation } from "./location";

/** A page of locations plus the opaque token for the next page (empty when exhausted). */
export interface ILocationPage {
    'locations': Array<ILocation>;
    'nextPageToken'?: string | null;
}
