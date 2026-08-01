import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listAccounts` returns under the same filters (M58 /
 * D-ObjectFacets): `totalCount` is what exhaustively paging that endpoint would yield, not an
 * estimate and not a page count.
 *
 * Every facet here partitions the result set — each counted row lands in exactly one bucket
 * of each distribution, so a distribution's counts sum to `totalCount`.
 *
 * There is deliberately NO facet over the IBAN. It is envelope-encrypted at rest with no
 * plaintext to group, and D-DataScope's aggregation rule forbids the surface regardless.
 *
 */
export interface IAccountStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
