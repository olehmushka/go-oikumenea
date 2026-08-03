import { IFacetBucket } from "./facetBucket";

/**
 * One facet's buckets, in chart order — for an enum, the declared CHECK-set order with
 * zero-count buckets included so a chart's shape is stable; for a ref, descending by count
 * with `(other)`/`(unknown)` last; for `foundedOn`, ascending by year.
 *
 */
export interface IFacetDistribution {
    'facet': string;
    'buckets': Array<IFacetBucket>;
}
