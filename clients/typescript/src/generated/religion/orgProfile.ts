import { IOrgClassification } from "./orgClassification";

/** The 1:1 faith attributes of a religious-body unit, with its classification tags. */
export interface IOrgProfile {
    'unitId': string;
    'orgKindId'?: string | null;
    'shortCode'?: string | null;
    'classifications': Array<IOrgClassification>;
    'createdAt': string;
    'updatedAt': string;
}
