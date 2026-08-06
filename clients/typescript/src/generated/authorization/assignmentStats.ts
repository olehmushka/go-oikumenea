import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listAssignments` returns under the same filters,
 * with the reach trim folded INTO the count (M58 ticket 6 / D-ObjectFacets).
 *
 * TWO arms. An instance admin counts every grant; anyone else counts only grants whose
 * target unit is within their `assignment.read` reach — the same question the `targetUnitId`
 * filter has always asked, asked for many units at once. The trim is computed over
 * `assignment.read` SPECIFICALLY and not over the `'%.read'` family that every other module's
 * reach predicate uses: generic read-reach is a strict superset, so borrowing it here would
 * widen a surface rather than describe it.
 *
 * ACTIVE ONLY. `listAssignments` returns active grants (`revokedAt` null) and this counts the
 * same population, so `totalCount` is a count of ACTIVE grants — not of rows in the grant
 * table. A revoked grant is a security artefact whose reachability is an authz read-surface
 * decision rather than a facet-vocabulary one, which is why the default stands and why there
 * is no `active` facet to make a one-bar chart out of.
 *
 */
export interface IAssignmentStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
