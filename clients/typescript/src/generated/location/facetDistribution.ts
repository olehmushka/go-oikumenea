import { IFacetBucket } from "./facetBucket";

/**
 * One facet's buckets, in chart order — descending by count, with `(other)`/`(unknown)` last.
 *
 */
export interface IFacetDistribution {
    'facet': string;
    'buckets': Array<IFacetBucket>;
}
