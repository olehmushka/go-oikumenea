import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listExternalOrgs` returns under the same filters
 * (M58 / D-ObjectFacets): `totalCount` is what exhaustively paging that endpoint would yield,
 * not an estimate and not a page count.
 *
 * Every facet here partitions the result set — each counted row lands in exactly one bucket
 * of each distribution, so a distribution's counts sum to `totalCount`.
 *
 */
export interface IExternalOrgStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
