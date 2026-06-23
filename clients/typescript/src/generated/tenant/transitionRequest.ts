import { UnitState } from "./unitState";

/** Transition a unit's lifecycle state (suspend/archive/restore). */
export interface ITransitionRequest {
    'toState': UnitState;
    'reason'?: string | null;
}
