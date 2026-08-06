import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listLocations` returns under the same arguments —
 * the dashboard half of the location facet vocabulary (M58 ticket 6 / D-ObjectFacets).
 *
 * ONE visibility arm, and no subject: a location carries no owner, no unit and no
 * public/shadow bit (D-Location — a referencing module owns the *meaning* of a place on its
 * own link), so `location.read` held anywhere is the whole gate and there is no second arm
 * for a visibility decision to make. That is languoid's shape — the ABSENCE of a decision —
 * and NOT the audit ledger's, which is a decision made entirely by which connection the
 * query runs on.
 *
 * THE SPATIAL WINDOW COUNTS. Unlike a tree-walk arg, a radius or bounding box is a predicate
 * over the listed table itself, so it narrows this aggregate exactly as it narrows the list
 * and the same mode precedence applies (`query` beats radius beats bbox). `totalCount`
 * therefore equals the rows exhaustively paging `listLocations` under these arguments would
 * return, in every one of the four modes.
 *
 */
export interface ILocationStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
