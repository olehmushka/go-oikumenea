/**
 * One bucket of a facet distribution (M58 / D-ObjectFacets).
 *
 * `key` is the bucket's stable, locale-agnostic identity — an enum value (`dialect`) or an open
 * code (`Eurasia`, `indo1319`) — and is exactly what you pass back as the corresponding list
 * filter. Two synthetic keys never name a real value: `(unknown)` is the NULL bucket and
 * `(other)` a top-N facet's collapsed tail; neither is a usable filter value.
 *
 * `label` carries a display name as a locale -> text map (D-i18n) and is present only for `ref`
 * buckets, whose keys are RIDs. The languoid type declares NO ref facet — a glottocode and a
 * macroarea are their own labels — so no bucket here carries one.
 *
 */
export interface IFacetBucket {
    'key': string;
    'label'?: { [key: string]: string } | null;
    'count': number;
}
