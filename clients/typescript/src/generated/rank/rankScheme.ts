import { IRankSystem } from "./rankSystem";

/** The whole scheme, systems -> categories -> types -> ranks, each level in seniority order. */
export interface IRankScheme {
    'systems': Array<IRankSystem>;
}
