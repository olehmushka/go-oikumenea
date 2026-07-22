import { ILinkRow } from "./linkRow";

/**
 * The queried object's neighborhood as a flat neighbor list — the graph-explorer shape
 * (search-around). depth=1 (default) is the direct neighborhood; depth=2 additionally returns
 * each direct neighbor's own neighbors (rows tagged hop=2, carrying viaRid). Depth is capped
 * at 2 (D-LinkTraversal depth-2, "full keyset frontier").
 *
 */
export interface INeighborhood {
    'rid': string;
    'neighbors': Array<ILinkRow>;
    'nextPageToken'?: string | null;
}
