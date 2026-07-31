import { IFacetBucket } from "./facetBucket";

/**
 * One facet's buckets, in chart order — for `rankId` the taxonomy rank's OWN ordinal (a rank
 * ladder re-sorted by frequency would destroy the only ordering that means anything), and for
 * every other facet descending by count with `(other)`/`(unknown)` last.
 *
 */
export interface IFacetDistribution {
    'facet': string;
    'buckets': Array<IFacetBucket>;
}
