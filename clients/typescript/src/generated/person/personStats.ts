import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listPersons` returns under the same filters, counted
 * INSIDE the visibility predicate (D-ObjectFacets): `totalCount` is what exhaustively paging
 * that list would yield, not an estimate and not a page count.
 *
 * A facet whose inherited read code the caller lacks is ABSENT from `facets` — never a zeroed
 * bucket and never a 403.
 *
 */
export interface IPersonStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
