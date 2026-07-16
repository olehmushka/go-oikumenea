import { ILinkRow } from "./linkRow";

/**
 * The queried object's depth-1 neighborhood as a flat neighbor list — the graph-explorer
 * shape (search-around). Depth>1 is a deliberate non-goal for this endpoint (review-2026-09).
 *
 */
export interface INeighborhood {
    'rid': string;
    'neighbors': Array<ILinkRow>;
    'nextPageToken'?: string | null;
}
