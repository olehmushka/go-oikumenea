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
    case "code":
      return { [f.params[0]]: bucketKey };
    case "date-range":
      if (f.buckets === "bands") return ageBandPatch(f, bucketKey, now);
      // Grain is read off the KEY's shape, not off a declaration: `2026-07-30` is a day, `2026-07` a
      // month, `1913` a year. Three grains now — year arrived with company/institution `foundedOn`
      // (M58 ticket 5), whose data spans a century and would otherwise be ~1500 mostly-empty months.
      if (/^\d{4}-\d{2}-\d{2}$/.test(bucketKey)) return dayPatch(f, bucketKey);
      if (/^\d{4}-\d{2}$/.test(bucketKey)) return monthPatch(f, bucketKey);
      return yearPatch(f, bucketKey);
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
  // Same argType branch dayPatch makes, and for the same reason: a DATETIME arg rejects a bare
  // calendar date outright (400), so without this every month segment of a datetime facet is a dead
  // link. It only surfaced with external_organization.asOf — the first month-grain DATETIME facet;
  // the earlier month facets (document.issuedOn/expiresOn, order.issuedOn) are calendar dates, and
  // audit's datetime facet buckets by DAY, so the two halves of this branch had never met.
  return f.argType === "datetime"
    ? { [f.params[0]]: dayBound(isoDate(first), false), [f.params[1]]: dayBound(isoDate(last), true) }
    : { [f.params[0]]: isoDate(first), [f.params[1]]: isoDate(last) };
}

/**
 * `1913` → the first and last day of that year, in the facet's own two args (M58 ticket 5).
 *
 * The same shape monthPatch has, including the datetime widening — not because a year-grain DATETIME
 * facet exists today (both are calendar dates), but because the month branch's absence of it is
 * exactly the bug ticket 2 found: every month segment of external_organization.asOf linked to a 400
 * for the whole of its existence, because monthPatch had never met a datetime facet. Writing the
 * branch once, here, is cheaper than discovering it again from a dead link.
 */
function yearPatch(f: FilterDef, key: string): FilterPatch | null {
  if (!/^\d{4}$/.test(key) || f.params.length < 2) return null;
  const first = `${key}-01-01`;
  const last = `${key}-12-31`;
  return f.argType === "datetime"
    ? { [f.params[0]]: dayBound(first, false), [f.params[1]]: dayBound(last, true) }
    : { [f.params[0]]: first, [f.params[1]]: last };
}

/** `2026-07-30` → that day's bounds, in whichever form the facet's args take. */
function dayPatch(f: FilterDef, key: string): FilterPatch | null {
  if (f.params.length < 2) return null;
  return f.argType === "datetime"
    ? { [f.params[0]]: dayBound(key, false), [f.params[1]]: dayBound(key, true) }
    : { [f.params[0]]: key, [f.params[1]]: key };
}

/**
 * A calendar day widened to one end of its RFC-3339 span, for an arg the contract types as a
 * DATETIME (audit's since/until). The upper bound is the last representable millisecond of the day
 * rather than the next midnight, because the endpoint's bounds are INCLUSIVE — `until` at the next
 * day's 00:00:00 would silently swallow one instant of the following day into every bucket.
 *
 * The one place this conversion lives: the filter control and the histogram's click-through must
 * agree exactly, or a clicked bar and a typed date would select different rows.
 */
export function dayBound(day: string, upper: boolean): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(day)) return day; // empty (a cleared input) or already a timestamp
  return upper ? `${day}T23:59:59.999Z` : `${day}T00:00:00.000Z`;
}

/** The inverse, for seeding a `<input type="date">` from whatever the URL holds. */
export function dayInput(value: string): string {
  const m = /^(\d{4}-\d{2}-\d{2})/.exec(value);
  return m ? m[1] : "";
}

/** `-P30D` → the ISO timestamp 30 days before `now`; anything else passes through unchanged. A
 *  dashboard's default window is written relatively so a shared link means "the last 30 days" when
 *  the reader opens it, not when the author did. */
export function resolveDefaultParam(value: string, now: Date): string {
  const m = /^-P(\d+)D$/.exec(value);
  return m ? plusDays(now, -Number(m[1])).toISOString() : value;
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
