import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listLanguages` returns under the same structural
 * filters (M58 / D-ObjectFacets), so `totalCount` equals the rows exhaustively paging
 * `listLanguages` would return.
 *
 */
export interface ILanguoidStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
