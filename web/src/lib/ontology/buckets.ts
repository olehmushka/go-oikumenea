// Bucket → filter patch (M57 ticket 3, D-ConsoleDashboards).
//
// "A chart segment and a list filter are the same act" is the load-bearing claim of the whole
// dashboard design, and this file is where it is either true or a lie. One pure function turns a
// bucket key into the URL patch that selects exactly the rows that bucket counted, so click-through
// cannot drift chart by chart — and every case that CANNOT be expressed as a filter returns null
// rather than an approximate link, because a link that lands on a different row set is worse than no
// link at all.
//
// Everything reads arity and names off FilterDef.params, never off the kind: unit.level is a
// numeric-range with ONE param, and membership's date range is effectiveFromAfter/Before rather than
// the derived …From/…To. The params array is the resolved truth (M56 ticket 4).

import type { FilterDef, ObjectTypeDef } from "./registry";
import { isSyntheticBucket } from "./stats";

/** `undefined` clears a param, exactly as exploreHref reads it. */
export type FilterPatch = Record<string, string | undefined>;

/**
 * The patch that narrows the current view to `bucketKey` of `facet`, or **null** when no filter
 * expresses it. Three things are deliberately unclickable:
 *
 *   - `(unknown)` — no arg says "the rows whose column is NULL". Every date/numeric bound EXCLUDES
 *     nulls (SQL three-valued logic), so the nearest link would return the complement of the bucket.
 *   - `(other)` — a top-N tail is "everything except these fifteen", which no single-value arg says.
 *   - a band whose facet ships only a scalar arg — unit.level's `level` matches ONE level, and the
 *     catalog buckets it in pairs. facets.md records levelMin/levelMax as additive and deferred
 *     "to when the bands are consumed"; consuming them is what earns them, so until then the bars
 *     are readable and inert rather than wrong.
 */
export function bucketPatch(
  def: ObjectTypeDef,
  facet: string,
  bucketKey: string,
  now: Date,
): FilterPatch | null {
  if (isSyntheticBucket(bucketKey)) return null;
  const f = (def.filters ?? []).find((x) => x.key === facet);
  if (!f) return null;

  switch (f.kind) {
    case "enum":
    case "bool":
    case "ref":
      return { [f.params[0]]: bucketKey };
    case "date-range":
      return f.buckets === "bands"
        ? ageBandPatch(f, bucketKey, now)
        : monthPatch(f, bucketKey);
    case "numeric-range":
      return numericBandPatch(f, bucketKey);
    default:
      return null;
  }
}

/** `[lo, hi]` inclusive from a band key: "25-34" → [25, 34]; "65+" → [65, null]. */
function parseBand(key: string): [number, number | null] | null {
  const open = /^(\d+)\+$/.exec(key);
  if (open) return [Number(open[1]), null];
  const closed = /^(\d+)-(\d+)$/.exec(key);
  if (closed) return [Number(closed[1]), Number(closed[2])];
  return null;
}

/**
 * The age→birthdate inversion. The aggregate buckets on
 * `EXTRACT(YEAR FROM age(current_date, birthdate))` — COMPLETED years — so band `[lo, hi]` is
 * `lo <= age <= hi`, and the filter args bound the birthdate itself:
 *
 *     age >= lo   ⟺  birthdate <= today − lo years
 *     age <= hi   ⟺  birthdate >  today − (hi+1) years   ⟺  birthdate >= today − (hi+1) years + 1 day
 *
 * Both list bounds are inclusive (`birthdate >= from AND birthdate <= to`), hence the +1 day. Getting
 * this off by a day is invisible on a chart and wrong in the list, which is why ticket 4 checks the
 * clicked bucket's count against the list it lands on rather than checking that it merely navigates.
 */
function ageBandPatch(f: FilterDef, key: string, now: Date): FilterPatch | null {
  const band = parseBand(key);
  if (!band || f.params.length < 2) return null;
  const [lo, hi] = band;
  const patch: FilterPatch = { [f.params[1]]: isoDate(minusYears(now, lo)) };
  if (hi !== null) patch[f.params[0]] = isoDate(plusDays(minusYears(now, hi + 1), 1));
  else patch[f.params[0]] = undefined; // "65+" is open-ended: clear any inherited lower bound
  return patch;
}

/** `2026-03` → the first and last day of that month, in the facet's own two args. */
function monthPatch(f: FilterDef, key: string): FilterPatch | null {
  const m = /^(\d{4})-(\d{2})$/.exec(key);
  if (!m || f.params.length < 2) return null;
  const year = Number(m[1]);
  const month = Number(m[2]);
  const first = new Date(Date.UTC(year, month - 1, 1));
  const last = new Date(Date.UTC(year, month, 0)); // day 0 of the next month = last of this one
  return { [f.params[0]]: isoDate(first), [f.params[1]]: isoDate(last) };
}

/** A numeric band needs a min/max PAIR; with a single scalar arg only a one-value band is expressible. */
function numericBandPatch(f: FilterDef, key: string): FilterPatch | null {
  const band = parseBand(key);
  if (!band) return null;
  const [lo, hi] = band;
  if (f.params.length >= 2) {
    const patch: FilterPatch = { [f.params[0]]: String(lo) };
    patch[f.params[1]] = hi === null ? undefined : String(hi);
    return patch;
  }
  return hi !== null && hi === lo ? { [f.params[0]]: String(lo) } : null;
}

// ── UTC date arithmetic (the DB runs in UTC; a local-time slip would move a band by a day) ──

function minusYears(d: Date, years: number): Date {
  const y = d.getUTCFullYear() - years;
  const m = d.getUTCMonth();
  // 29 Feb minus a non-leap span: JS would roll into 1 March, moving the boundary a day the wrong
  // way. Clamp to the last day of the target month instead, which is what age() compares against.
  const lastOfMonth = new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
  return new Date(Date.UTC(y, m, Math.min(d.getUTCDate(), lastOfMonth)));
}

/** Exported for the derived tiles that bound a window rather than pick a bucket (expiring soon). */
export function plusDays(d: Date, days: number): Date {
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + days));
}

export function isoDate(d: Date): string {
  return d.toISOString().slice(0, 10);
}
