import { IUnitRef } from "./unitRef";

/** An ordered list of unit references (ancestors), nearest first. */
export interface IUnitRefList {
    'units': Array<IUnitRef>;
}
