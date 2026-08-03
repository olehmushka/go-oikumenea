import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listInstitutions` returns under the same filters,
 * with the shadow gate folded INTO the count (M58 ticket 5 / D-ObjectFacets).
 *
 * An institution IS a `university`-domain tenant ORGANIZATION plus a sidecar (M41 /
 * D-UnifiedOrgGraph), so it carries that organization's public/shadow bit and this count
 * applies the organization gate: `public OR reachable`, where reach is DERIVED from unit reach
 * (D-VisibilityScope, amended M58 ticket 4) — an organization is visible when any of its live
 * units is in the subject's reach. Folded into SQL rather than applied to the page, because
 * trimming after the fact is right for a page and wrong for a count. `totalCount` therefore
 * equals the rows exhaustively paging `listInstitutions` under these filters would return.
 *
 */
export interface IInstitutionStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
