import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listOrders` returns under the same filters, counted
 * INSIDE the caller's readable reach on the issuing unit (D-ObjectFacets): `totalCount` is what
 * exhaustively paging that list would yield.
 *
 * Note that `orderTypeId` counts ORDER ITEMS' types: an order carrying two different effects
 * appears in two buckets, so that one distribution deliberately does not sum to `totalCount`.
 *
 */
export interface IOrderStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
