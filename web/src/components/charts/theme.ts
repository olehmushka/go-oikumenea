// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// The chart system's shared parameters (M57 ticket 3, D-ConsoleDashboards): one palette, one ink
// scale, one set of geometry constants and one number formatter, so five primitives and five
// dashboards read as ONE system rather than five drawings.
//
// COLOUR IS ASSIGNED BY JOB, not by taste:
//   - MAGNITUDE (a bar, a histogram, a sparkline — one series, sorted or ordered) takes ONE hue.
//     Painting each bar a different colour encodes nothing and spends the categorical scale.
//   - IDENTITY (a donut's slices, the pyramid's two wings — unordered categories side by side) takes
//     the categorical slots IN FIXED ORDER, never cycled. A 7th slice folds into `(other)`.
//   - STATUS (revoked, draft, past-due) takes the console's own semantic tones, the same ones
//     `Pill`/`statusTone` paint a table cell with — a revoked order must not be red here and amber
//     there. Status hues are RESERVED: they are never handed out as "categorical slot 4".
//
// The categorical slots are validated, not eyeballed: the six below pass the lightness band, the
// chroma floor, adjacent-pair CVD separation (worst ΔE 9.1, protan) and the normal-vision floor
// (worst ΔE 19.6) on a light surface. Three of them sit below 3:1 contrast against white, which
// obliges the RELIEF RULE — every chart here ships visible direct labels or a legend carrying the
// count, so identity is never colour-alone and a low-contrast fill is never the only carrier.
// Re-run the check before adding a slot; do not reason about ΔE by hand.
//
// The console is light-only (`globals.css` sets `color-scheme: light`), so there is no dark step set
// to keep in sync. Adding a dark theme means re-validating these hues against the dark surface — an
// automatic flip is not a palette.

import type { Tone } from "@/lib/ontology/registry";

/** Categorical slots, in fixed assignment order. Slot 1 doubles as the single-hue magnitude fill. */
export const SERIES = [
  "#2a78d6", // blue
  "#eb6834", // orange
  "#1baf7a", // aqua
  "#eda100", // yellow
  "#e87ba4", // magenta
  "#008300", // green
] as const;

/** The one hue a single-series magnitude chart uses. */
export const MAGNITUDE = SERIES[0];

/** Categorical slot `i`, folded rather than cycled: past the last slot a caller must use `(other)`. */
export function seriesColor(i: number): string {
  return SERIES[Math.min(i, SERIES.length - 1)];
}

/**
 * Status hues, matching `Pill`'s tones so one status wears one colour across the console. `amber` is
 * the 600 step rather than Pill's 500 background: a fill needs the darker step to clear 3:1 against
 * white, where a pill only tints behind dark text.
 */
export const TONE_FILL: Record<Tone, string> = {
  slate: "#94a3b8",
  green: "#16a34a",
  amber: "#d97706",
  red: "#dc2626",
  indigo: "#4f46e5",
};

/** Text and chrome ink. Values, labels and legends wear these — never a series colour. */
export const INK = {
  primary: "#0f172a", // slate-900
  secondary: "#475569", // slate-600
  muted: "#94a3b8", // slate-400
  grid: "#e2e8f0", // slate-200
  surface: "#ffffff",
};

/**
 * The fill for `(unknown)` and `(other)`. Deliberately a neutral grey and deliberately NOT a
 * categorical slot: neither names a real value, neither is clickable (no filter expresses "the rows
 * whose column is NULL"), and a synthetic bucket that looked like a category would invite reading it
 * as one. `(unknown)` is a data-quality signal — muted, never hidden.
 */
export const SYNTHETIC_FILL = "#cbd5e1"; // slate-300

/** Shared geometry. Charts are fixed-viewBox SVGs scaled by CSS, so nothing measures the DOM. */
export const GEO = {
  rowHeight: 26, // one horizontal bar row
  barGap: 6, // the 2px+ surface gap between adjacent fills, at row scale
  radius: 3, // rounded data-end
  vbWidth: 720, // the viewBox width every chart shares
  labelGutter: 190, // left label column of a horizontal bar chart
  valueGutter: 56, // right value column
  fontSize: 12,
};

// ── formatting ──────────────────────────────────────────────────────────────

// ISO 639-3 (the API's and the console's locale identifiers) → the BCP-47 tag Intl understands.
// Kept here rather than in i18n.ts because number formatting is the only consumer: the label maps
// are keyed by the ISO 639-3 code and must stay that way.
const INTL_TAG: Record<string, string> = { eng: "en", ukr: "uk", spa: "es", por: "pt" };

/** Group-separated integer in the active UI locale (1 234 567 / 1,234,567). */
export function fmtInt(n: number, locale: string): string {
  return new Intl.NumberFormat(INTL_TAG[locale] ?? "en").format(n);
}

/** A share of the whole, for legends and derived tiles. One decimal below 10 %, none above. */
export function fmtPct(part: number, whole: number, locale: string): string {
  if (whole <= 0) return "—";
  const p = (part / whole) * 100;
  return new Intl.NumberFormat(INTL_TAG[locale] ?? "en", {
    minimumFractionDigits: p < 10 && p > 0 ? 1 : 0,
    maximumFractionDigits: p < 10 && p > 0 ? 1 : 0,
  }).format(p) + " %";
}

/** `2026-03` → `Mar 2026` in the active locale; anything else passes through unchanged. */
export function fmtMonth(key: string, locale: string): string {
  const m = /^(\d{4})-(\d{2})$/.exec(key);
  if (!m) return key;
  const d = new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, 1));
  return new Intl.DateTimeFormat(INTL_TAG[locale] ?? "en", {
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  }).format(d);
}

/** `2026-07-30` → `30 Jul` in the active locale (the year lives on the axis, not on every tick). */
export function fmtDay(key: string, locale: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(key)) return key;
  return new Intl.DateTimeFormat(INTL_TAG[locale] ?? "en", {
    day: "numeric",
    month: "short",
    timeZone: "UTC",
  }).format(new Date(`${key}T00:00:00Z`));
}

/** A dateTrunc bucket key rendered at whichever grain it came back at. */
export function fmtTimeBucket(key: string, locale: string): string {
  return /^\d{4}-\d{2}-\d{2}$/.test(key) ? fmtDay(key, locale) : fmtMonth(key, locale);
}

/** Truncate a label to fit a fixed gutter; the full text always survives in the mark's <title>. */
export function clip(text: string, max: number): string {
  return text.length <= max ? text : `${text.slice(0, max - 1)}…`;
}

/**
 * One drawable mark. The dashboard resolves buckets into these ONCE — display label, click target
 * and colour — so a primitive never has to know about locale maps, synthetic keys or filter args.
 */
export type Segment = {
  /** the bucket key: an enum value, a RID, a band, `YYYY-MM`, or a synthetic `(…)` key */
  key: string;
  /** resolved display text (already locale-picked / translated) */
  label: string;
  count: number;
  /** the same-URL-plus-one-filter link; ABSENT means this segment is not expressible as a filter */
  href?: string;
  /** overrides the chart's own colour choice (status tones, past-due) */
  color?: string;
};
