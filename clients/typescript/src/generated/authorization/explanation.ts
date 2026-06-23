import { IContribution } from "./contribution";

/** Why a decision was reached. For ALLOW, the contributing grants (may be several across graphs); for DENY, the reason. */
export interface IExplanation {
    'contributions': Array<IContribution>;
    'denyReason'?: string | null;
}
