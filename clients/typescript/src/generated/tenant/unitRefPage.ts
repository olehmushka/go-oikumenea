import { IUnitRef } from "./unitRef";

/** A page of unit references (descendants) plus the opaque next-page token. */
export interface IUnitRefPage {
    'units': Array<IUnitRef>;
    'nextPageToken'?: string | null;
}
