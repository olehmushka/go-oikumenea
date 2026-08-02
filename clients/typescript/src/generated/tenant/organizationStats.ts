import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listOrganizations` returns under the same filters,
 * with the shadow gate folded INTO the count (M58 / D-ObjectFacets).
 *
 * For an organization that gate is `visibility = 'public'` and NOT the reach predicate the
 * unit dashboard uses: a role assignment's `target_unit_id` FKs `tenant_units`, so an
 * organization RID can never appear in any subject's readable reach, and a shadow
 * organization is visible to an instance admin and to nobody else. That is exactly what
 * `gateUnits` leaves on the list, which is why `totalCount` equals the rows exhaustively
 * paging `listOrganizations` under these filters would return.
 *
 */
export interface IOrganizationStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
