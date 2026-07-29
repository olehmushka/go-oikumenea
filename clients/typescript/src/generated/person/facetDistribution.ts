import { IFacetBucket } from "./facetBucket";

/**
 * One facet's buckets, in chart order — for an enum, the declared CHECK-set order with
 * zero-count buckets included, so a chart's shape is stable across filterings; for bands, the
 * declared band order; for a ref, descending by count (or the scheme's own order where it has
 * one, e.g. rank seniority) with `(other)`/`(unknown)` last.
 *
 */
export interface IFacetDistribution {
    'facet': string;
    'buckets': Array<IFacetBucket>;
}
