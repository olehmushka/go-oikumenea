import { IUnit } from "./unit";

/** A page of units plus the opaque token for the next page (empty when exhausted). */
export interface IUnitPage {
    'units': Array<IUnit>;
    'nextPageToken'?: string | null;
}
