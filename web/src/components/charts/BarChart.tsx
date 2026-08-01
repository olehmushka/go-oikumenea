// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Magnitude over a set of categories (M57 ticket 3) — the workhorse of the module dashboards:
// top-15 ref distributions, the rank seniority profile, the level width profile, the order type mix.
//
// ONE hue, because there is one series: the bars' lengths carry the comparison, and colouring each
// bar differently would spend the categorical scale on nothing. A `Segment.color` overrides per bar
// where the value carries STATUS (facets.md tones revoked red, draft amber) — status is a different
// job from identity, so it is allowed to break the single-hue rule and nothing else is.
//
// Every bar is an <a> to the same explorer URL with one more filter applied, which is the whole
// design: a chart segment and a list filter are the same act (D-ConsoleDashboards). A segment with no
// href — `(unknown)`, `(other)` — renders as a plain mark: no filter value expresses "the rows whose
// column is NULL", so a link there would silently drop the filter and show the unfiltered list.
//
// Server Component: plain SVG, no client JS. The readout is a native <title> tooltip plus the direct
// value label every bar already carries, which is also what discharges the palette's contrast relief
// rule — the number is never encoded by fill alone.

import { scaleBand, scaleLinear } from "@visx/scale";
import { BarRounded } from "@visx/shape";
import { NoData } from "./ChartCard";
import { clip, fmtInt, GEO, INK, MAGNITUDE, SYNTHETIC_FILL, type Segment } from "./theme";

export function BarChart({
  segments,
  locale,
  orientation = "horizontal",
  /** the label under every value in the <title> readout, e.g. "persons" */
  unit,
}: {
  segments: Segment[];
  locale: string;
  orientation?: "horizontal" | "vertical";
  unit?: string;
}) {
  if (segments.length === 0) return <NoData />;
  const max = Math.max(...segments.map((s) => s.count), 1);
  return orientation === "vertical" ? (
    <VerticalBars segments={segments} locale={locale} max={max} unit={unit} />
  ) : (
    <HorizontalBars segments={segments} locale={locale} max={max} unit={unit} />
  );
}

const fillOf = (s: Segment): string => s.color ?? (s.href ? MAGNITUDE : SYNTHETIC_FILL);

/**
 * A hairline outline for a fill too light to have an edge against the white surface — the relief the
 * palette's own contrast rule demands, applied where the fill is DATA rather than a chosen hue
 * (M58 ticket 3: the vehicle colour chart paints `white` and `silver` bars). Every validated palette
 * slot clears the bar already, so this is inert for every other chart.
 *
 * Relative luminance per WCAG 2.x; the 0.75 cut is above the lightest categorical slot and below a
 * plausible "silver".
 */
function strokeOf(s: Segment): string | undefined {
  const m = /^#([0-9a-f]{6})$/i.exec(s.color ?? "");
  if (!m) return undefined;
  const n = parseInt(m[1], 16);
  const lin = (c: number) => {
    const v = c / 255;
    return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
  };
  const L =
    0.2126 * lin((n >> 16) & 0xff) + 0.7152 * lin((n >> 8) & 0xff) + 0.0722 * lin(n & 0xff);
  return L > 0.75 ? INK.grid : undefined;
}
const readout = (s: Segment, locale: string, unit?: string): string =>
  `${s.label}: ${fmtInt(s.count, locale)}${unit ? ` ${unit}` : ""}`;

/** Long category names (units, countries, ranks) need a label column, so bars run left→right. */
function HorizontalBars({
  segments,
  locale,
  max,
  unit,
}: {
  segments: Segment[];
  locale: string;
  max: number;
  unit?: string;
}) {
  const { rowHeight, barGap, radius, vbWidth, labelGutter, valueGutter, fontSize } = GEO;
  const plot = vbWidth - labelGutter - valueGutter;
  const height = segments.length * rowHeight + 4;
  const x = scaleLinear<number>({ domain: [0, max], range: [0, plot] });

  return (
    <svg
      viewBox={`0 0 ${vbWidth} ${height}`}
      width="100%"
      height={height}
      role="img"
      className="overflow-visible"
    >
      {segments.map((s, i) => {
        const y = i * rowHeight + 2;
        const w = Math.max(x(s.count), s.count > 0 ? 2 : 0);
        const body = (
          <g>
            <title>{readout(s, locale, unit)}</title>
            <text
              x={labelGutter - 8}
              y={y + rowHeight / 2}
              textAnchor="end"
              dominantBaseline="middle"
              fontSize={fontSize}
              fill={s.href ? INK.secondary : INK.muted}
            >
              {clip(s.label, 26)}
            </text>
            <BarRounded
              x={labelGutter}
              y={y + barGap / 2}
              width={w}
              height={rowHeight - barGap}
              radius={radius}
              right
              fill={fillOf(s)}
              stroke={strokeOf(s)}
            />
            <text
              x={labelGutter + w + 6}
              y={y + rowHeight / 2}
              dominantBaseline="middle"
              fontSize={fontSize}
              fill={INK.secondary}
            >
              {fmtInt(s.count, locale)}
            </text>
          </g>
        );
        return s.href ? (
          <a key={s.key} href={s.href} className="chart-mark">
            {body}
          </a>
        ) : (
          <g key={s.key}>{body}</g>
        );
      })}
    </svg>
  );
}

/** Short, ORDERED categories (levels, rank seniority) read left→right, so the bars stand up. */
function VerticalBars({
  segments,
  locale,
  max,
  unit,
}: {
  segments: Segment[];
  locale: string;
  max: number;
  unit?: string;
}) {
  const { vbWidth, fontSize } = GEO;
  const plotHeight = 180;
  const labelBand = 74; // room for the rotated tick labels
  const height = plotHeight + labelBand;
  const x = scaleBand<string>({
    domain: segments.map((s) => s.key),
    range: [0, vbWidth],
    padding: 0.25,
  });
  const y = scaleLinear<number>({ domain: [0, max], range: [plotHeight, 0] });
  const bw = x.bandwidth();

  return (
    <svg viewBox={`0 0 ${vbWidth} ${height}`} width="100%" height={height} role="img">
      <line x1={0} x2={vbWidth} y1={plotHeight} y2={plotHeight} stroke={INK.grid} strokeWidth={1} />
      {segments.map((s) => {
        const bx = x(s.key) ?? 0;
        const top = y(s.count);
        const h = Math.max(plotHeight - top, s.count > 0 ? 2 : 0);
        const body = (
          <g>
            <title>{readout(s, locale, unit)}</title>
            <BarRounded
              x={bx}
              y={plotHeight - h}
              width={bw}
              height={h}
              radius={GEO.radius}
              top
              fill={fillOf(s)}
              stroke={strokeOf(s)}
            />
            <text
              x={bx + bw / 2}
              y={plotHeight - h - 5}
              textAnchor="middle"
              fontSize={fontSize - 1}
              fill={INK.secondary}
            >
              {fmtInt(s.count, locale)}
            </text>
            <text
              x={bx + bw / 2}
              y={plotHeight + 10}
              textAnchor="end"
              fontSize={fontSize - 1}
              fill={s.href ? INK.secondary : INK.muted}
              transform={`rotate(-35 ${bx + bw / 2} ${plotHeight + 10})`}
            >
              {clip(s.label, 22)}
            </text>
          </g>
        );
        return s.href ? (
          <a key={s.key} href={s.href} className="chart-mark">
            {body}
          </a>
        ) : (
          <g key={s.key}>{body}</g>
        );
      })}
    </svg>
  );
}
