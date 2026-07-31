import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listTaxa` returns under the same filters (M58 /
 * D-ObjectFacets): `totalCount` is what exhaustively paging that endpoint would yield, not an
 * estimate and not a page count.
 *
 * TWO of the four facets do NOT partition the result set, and this is a property of a
 * taxonomy rather than an inaccuracy:
 *
 * - `subtree` counts each taxon under EVERY ancestor it has, because that is what makes the
 *   chart drillable — a bucket's count is its whole subtree size and clicking it returns
 *   exactly those rows, then re-grouping within them yields that subtree's own internal
 *   nodes, recursively, all the way down.
 * - `classification` counts EFFECTIVE theism tags, resolved to the nearest declaring ancestor
 *   (what `getEffectiveClassifications` returns), and a taxon may carry several.
 *
 * So for those two, the buckets' counts SUM TO MORE than `totalCount`. What holds for every
 * facet without exception is the property the vocabulary actually rests on: a bucket's count
 * equals the number of rows `listTaxa` returns under that bucket's own filter.
 *
 */
export interface ITaxonStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
