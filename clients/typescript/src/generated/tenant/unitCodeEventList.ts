import { IUnitCodeEvent } from "./unitCodeEvent";

/** A unit's code-change history, newest first. */
export interface IUnitCodeEventList {
    'events': Array<IUnitCodeEvent>;
}
