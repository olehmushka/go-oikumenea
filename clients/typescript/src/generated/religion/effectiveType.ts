import { IClassification } from "./classification";

/** A unit's resolved religion-type, with the source that supplied it (nearest-declared-wins). */
export interface IEffectiveType {
    'unitId': string;
    'classifications': Array<IClassification>;
    /** One of "unit" (own override), "taxon:<code>" (inherited), or "none". */
    'source': string;
}
