import { IPosition } from "./position";

/** A page of positions plus the opaque token for the next page (empty when exhausted). */
export interface IPositionPage {
    'positions': Array<IPosition>;
    'nextPageToken'?: string | null;
}
