import { IFacetDistribution } from "./facetDistribution";

/**
 * Facet distributions over the SAME set `listEnrollments` returns under the same filters, with
 * the holder read-scope folded INTO the count (M58 ticket 7 / D-ObjectFacets).
 *
 * An enrollment carries no unit of its own — it is scoped THROUGH ITS HOLDER
 * (D-PersonReadScope), exactly as a document is: an instance admin counts everything, and
 * everyone else counts the enrollments of the people they may read (a person is readable when
 * they hold an active membership in a unit of the caller's reach). Folded into SQL rather than
 * applied to the page, because trimming after the fact is right for a page and wrong for a
 * count. `totalCount` therefore equals the rows exhaustively paging `listEnrollments` under
 * these filters would return.
 *
 * The `degreeLevelId` distribution is ordered by ISCED LEVEL, not by count, and includes
 * levels with no enrollments at all: it is a scale, and a scale re-sorted by frequency reads
 * as if a doctorate ranked below a bachelor's. It carries no `(other)` bucket for the same
 * reason — every level is named.
 *
 */
export interface IEnrollmentStats {
    'totalCount': number;
    'facets': Array<IFacetDistribution>;
}
