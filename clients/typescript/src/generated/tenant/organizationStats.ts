import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listOrganizations` returns under the same filters,
 * with the shadow gate folded INTO the count (M58 / D-ObjectFacets).
 *
 * For an organization that gate is `public OR reachable`, where reach is DERIVED from unit
 * reach (M58 ticket 4 follow-up, amending D-VisibilityScope): an organization is visible when
 * any of its live units is in the subject's reach. It is not the unit dashboard's predicate
 * copied — a role assignment's `target_unit_id` FKs `tenant_units`, so an organization RID can
 * never appear in a readable-unit set and that predicate would match nothing here. It is
 * exactly what `gateOrgs` leaves on the list, which is why `totalCount` equals the rows
 * exhaustively paging `listOrganizations` under these filters would return.
 *
 */
export interface IOrganizationStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
