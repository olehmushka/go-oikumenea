import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listDocuments` returns under the same filters, counted
 * INSIDE the holder read-scope semi-join (D-ObjectFacets / D-PersonReadScope): a document whose
 * holder the caller cannot read is not counted at all, rather than counted and then trimmed.
 *
 */
export interface IDocumentStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
