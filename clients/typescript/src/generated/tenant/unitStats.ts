import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listUnits` returns under the same filters, with the
 * shadow-visibility gate folded INTO the count (D-ObjectFacets): a shadow unit outside the
 * caller's reach is not counted at all, rather than counted and then trimmed.
 *
 */
export interface IUnitStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
