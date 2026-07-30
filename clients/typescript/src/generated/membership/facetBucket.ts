/**
 * One bucket of a facet distribution (M57 / D-ObjectFacets).
 *
 * `key` is the bucket's stable, locale-agnostic identity — an enum value, a `YYYY-MM` month, a
 * band, `true`/`false`, or a RID for a `ref` facet — and is exactly what you pass back as the
 * corresponding list filter, which is what makes a chart segment and a filter the same act.
 * Two synthetic keys never name a real value: `(unknown)` is the NULL bucket (mandatory for a
 * nullable column, so the gap is visible rather than dropped) and `(other)` is a top-N facet's
 * collapsed tail; neither is a usable filter value.
 *
 * `label` carries the object's display name as a locale → text map (D-i18n — all locales in
 * every response) and is present only for `ref` buckets, whose keys are RIDs. Best effort: an
 * id with no resolvable name simply carries no label.
 *
 */
export interface IFacetBucket {
    'key': string;
    'label'?: { [key: string]: string } | null;
    'count': number;
}
