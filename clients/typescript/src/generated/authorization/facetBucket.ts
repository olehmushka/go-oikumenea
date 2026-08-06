/**
 * One bucket of a facet distribution (M57 / D-ObjectFacets).
 *
 * `key` is the bucket's stable, locale-agnostic identity — a `scope` value (`unit`) or a RID
 * for a `ref` facet — and is exactly what you pass back as the corresponding list filter. Two
 * synthetic keys never name a real value: `(unknown)` is the NULL bucket and `(other)` a
 * top-N facet's collapsed tail; neither is a usable filter value.
 *
 * `label` carries the object's display name as a locale → text map (D-i18n) and is present
 * only for `ref` buckets, whose keys are RIDs. Best effort: an id with no resolvable name
 * carries no label.
 *
 */
export interface IFacetBucket {
    'key': string;
    'label'?: { [key: string]: string } | null;
    'count': number;
}
