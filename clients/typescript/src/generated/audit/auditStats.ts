import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `query` returns under the same filters (M58 /
 * D-ObjectFacets): `totalCount` is what exhaustively paging that endpoint would yield, not an
 * estimate and not a page count.
 *
 * Visibility is the row-level security policy on `audit_log`, exactly as it is for `query`:
 * entries whose unit is outside the caller's reach are not counted, and NULL-unit (system /
 * instance-plane) entries are counted only for an instance admin. There is no separate scoped
 * query because there is no separate scoped read.
 *
 * The ledger is month-partitioned and unbounded, so `since`/`until` are the only lever that
 * limits how much of it a dashboard touches — the console sends a default window rather than
 * this endpoint applying one, so that `totalCount` always describes exactly the filters in the
 * request.
 *
 */
export interface IAuditStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
