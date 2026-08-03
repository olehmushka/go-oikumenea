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
 * The inclusive `YYYY-MM-DD` sequence covering the buckets present — `monthSpan`'s day-grain sibling,
 * for the audit ledger's per-day histogram. Same reason and same bounded loop: a gap left unfilled
 * compresses a quiet fortnight into nothing, and a gap-filler over an unbounded ledger must not spin.
 * The cap is generous enough for a multi-year window and finite either way.
 */
export function daySpan(keys: string[]): string[] {
  const days = keys.filter((k) => /^\d{4}-\d{2}-\d{2}$/.test(k)).sort();
  if (days.length < 2) return days;
  const out: string[] = [];
  const cursor = new Date(`${days[0]}T00:00:00Z`);
  const last = days[days.length - 1];
  for (let i = 0; i < 1500; i++) {
    const key = cursor.toISOString().slice(0, 10);
    out.push(key);
    if (key === last) break;
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  return out;
}

/**
 * The maximum number of YEAR buckets worth densifying. A dense axis exists to make a GAP visible, and
 * past a screenful of bars it stops showing gaps and starts showing nothing else.
 *
 * The number is not arbitrary: the widest chart that already ships is the vehicle fleet-age histogram
 * at ~54 month bars, so this is the same visual budget expressed in the new grain.
 */
const MAX_DENSE_YEARS = 60;

/**
 * The inclusive `YYYY` sequence covering the buckets present — the third grain (M58 ticket 5), for the
 * company/institution founding histograms. Same reason as its two siblings: a founding curve with the
 * dormant decades omitted reads as steady activity, which is the opposite of what it shows.
 *
 * UNLIKE its siblings it gives up past `MAX_DENSE_YEARS` and returns the populated years instead, and
 * that is the one thing about this function found by running it rather than by writing it: the seeded
 * institutions span 1661–2016, so the dense axis rendered **356 bars for 8 rows** — every one of them
 * a live link, 348 of them empty. A month histogram covers an operational window and cannot do that;
 * a founding histogram covers however long the institution has existed.
 *
 * What is lost past the threshold is the visibility of gaps, and at that width nobody was reading the
 * gaps anyway. What is kept is every property the vocabulary depends on: each bar is still a real
 * year, still counts what its own filter returns, and still sums to `totalCount`.
 */
export function yearSpan(keys: string[]): string[] {
  const years = keys.filter((k) => /^\d{4}$/.test(k)).sort();
  if (years.length < 2) return years;
  const first = Number(years[0]);
  const last = Number(years[years.length - 1]);
  if (last - first + 1 > MAX_DENSE_YEARS) return years;
  const out: string[] = [];
  for (let y = first; y <= last; y++) out.push(String(y).padStart(4, "0"));
  return out;
}

/** The dense axis for a dateTrunc distribution, whichever grain it came back at. */
export function timeSpan(keys: string[]): string[] {
  if (keys.some((k) => /^\d{4}-\d{2}-\d{2}$/.test(k))) return daySpan(keys);
  if (keys.some((k) => /^\d{4}-\d{2}$/.test(k))) return monthSpan(keys);
  return yearSpan(keys);
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
