import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listMemberships` returns under the same filters,
 * counted INSIDE the caller's readable reach (D-ObjectFacets): `totalCount` is what
 * exhaustively paging that list would yield, not an estimate and not a page count.
 *
 * Like the list, it applies NO implicit status filter — the unfiltered total is the honest
 * one, and it agrees with its own status distribution by construction.
 *
 */
export interface IMembershipStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
