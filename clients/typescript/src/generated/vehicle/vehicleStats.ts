import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listVehicles` returns under the same filters
 * (M58 / D-ObjectFacets): `totalCount` is what exhaustively paging that endpoint would yield,
 * not an estimate and not a page count.
 *
 * Every facet here partitions the result set — each counted row lands in exactly one bucket
 * of each distribution, so a distribution's counts sum to `totalCount`. `registrationCountry`
 * partitions too, and does so BECAUSE it is confined to the one ACTIVE registration a vehicle
 * may hold; the ownership history behind it is one-to-many and would not.
 *
 */
export interface IVehicleStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
