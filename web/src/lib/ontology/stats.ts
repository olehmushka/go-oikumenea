// The stats response as the console reads it (M57 ticket 3, D-ObjectFacets).
//
// Isomorphic and pure — types plus the three transformations a dashboard needs that are NOT the
// API's job:
//
//   - TELLING ABSENT FROM EMPTY. A facet the caller may not read is OMITTED from the response (rule
//     2: never a zeroed bucket, never a 403), so `distribution()` returns undefined for "you cannot
//     see this" and an empty array for "there is nothing here". The dashboard draws nothing at all in
//     the first case and an empty state in the second; collapsing them would either invent a zero the
//     caller is not entitled to read, or hide a real emptiness.
//   - FILLING THE GAPS in a date_trunc facet. The endpoint deliberately emits no zero-fill ("inventing
//     empty months between the extremes is the chart's job, not the API's"), and a histogram that
//     closes its own gaps compresses a two-year lull into nothing.
//   - FOLDING A LONG TAIL for a donut. Fifteen slices are not a composition; past six the palette has
//     no more identity slots, and the honest move is to fold the rest into `(other)` — which is also
//     why the fold reports itself, so a reader knows the ring is not the whole set.
//
// Nothing here filters, re-weights or drops a counted row: every fold preserves the sum, because the
// endpoint's invariant is that every counted row lands in exactly one bucket.

import type { Segment } from "@/components/charts/theme";

export type StatsBucket = {
  key: string;
  /** ref buckets only: locale → text (D-i18n). Best effort — an unresolved id carries none. */
  label?: Record<string, string> | null;
  count: number;
};

export type FacetDistribution = { facet: string; buckets: StatsBucket[] };

export type StatsResponse = { totalCount: number; facets: FacetDistribution[] };

/** The two synthetic bucket keys (pkg/stats). Neither names a real value; neither is a filter. */
export const BUCKET_UNKNOWN = "(unknown)";
export const BUCKET_OTHER = "(other)";

export function isSyntheticBucket(key: string): boolean {
  return key === BUCKET_UNKNOWN || key === BUCKET_OTHER;
}

/** The facet's buckets, or `undefined` when the facet is absent — i.e. the caller may not read it. */
export function distribution(res: StatsResponse | null, facet: string): StatsBucket[] | undefined {
  return res?.facets.find((f) => f.facet === facet)?.buckets;
}

/** Split the `(unknown)` bucket out of a distribution: it is a number, never a point on a time axis. */
export function splitUnknown(buckets: StatsBucket[]): {
  rest: StatsBucket[];
  unknown?: StatsBucket;
} {
  return {
    rest: buckets.filter((b) => b.key !== BUCKET_UNKNOWN),
    unknown: buckets.find((b) => b.key === BUCKET_UNKNOWN),
  };
}

/**
 * The inclusive `YYYY-MM` sequence covering the buckets present, so a histogram's axis is time rather
 * than a list of the months that happen to be populated. Non-month keys are returned untouched — a
 * band facet is already dense by construction.
 */
export function monthSpan(keys: string[]): string[] {
  const months = keys.filter((k) => /^\d{4}-\d{2}$/.test(k)).sort();
  if (months.length < 2) return months;
  const out: string[] = [];
  const [firstY, firstM] = months[0].split("-").map(Number);
  const last = months[months.length - 1];
  let y = firstY;
  let m = firstM;
  // Guard the loop as well as the condition: a corrupt key must not spin forever server-side.
  for (let i = 0; i < 1200; i++) {
    const key = `${String(y).padStart(4, "0")}-${String(m).padStart(2, "0")}`;
    out.push(key);
    if (key === last) break;
    m += 1;
    if (m > 12) {
      m = 1;
      y += 1;
    }
  }
  return out;
}

/**
 * Keep the `max` largest segments and fold the rest — including any pre-existing `(other)` — into one
 * inert `(other)`. Order is preserved for the survivors (an enum's CHART order, a ref's count order);
 * the fold always lands last. Returns `folded` so the chart can say so.
 */
export function foldTail(
  segments: Segment[],
  max: number,
  otherLabel: string,
): { segments: Segment[]; folded: number } {
  if (segments.length <= max) return { segments, folded: 0 };
  const ranked = [...segments].sort((a, b) => b.count - a.count).slice(0, max);
  const keep = new Set(ranked.map((s) => s.key));
  const kept = segments.filter((s) => keep.has(s.key) && s.key !== BUCKET_OTHER);
  const tail = segments.filter((s) => !keep.has(s.key) || s.key === BUCKET_OTHER);
  const sum = tail.reduce((n, s) => n + s.count, 0);
  return {
    segments: [...kept, { key: BUCKET_OTHER, label: otherLabel, count: sum }],
    folded: tail.length,
  };
}

/** The `facets` CSV: what the dashboard DRAWS, never everything it could ask for. The difference is
 *  ~11 s against ~3 s at root reach (review-2026-07 § M57 ticket 2), so this is the console's one
 *  real performance lever. An EMPTY list must never be sent as `facets=` — that means "count only". */
export function facetsCsv(keys: string[]): string {
  return Array.from(new Set(keys)).join(",");
}
