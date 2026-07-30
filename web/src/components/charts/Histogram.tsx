// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Counts over a continuous axis (M57 ticket 3): the membership intake curve, orders per month,
// documents by expiry month, the person age structure when it is not split into a pyramid.
//
// The bars TOUCH (a small surface gap only), because the axis is continuous — a histogram whose bars
// float apart reads as unordered categories. For the same reason nothing here is re-sorted: the
// buckets arrive in the API's chronological/band order and are drawn in it.
//
// Empty interior buckets are the CALLER's job to supply (see fillMonths in lib/ontology/stats): the
// stats endpoint deliberately emits no zero-fill for a date_trunc facet — "inventing empty months
// between the extremes is the chart's job, not the API's" — and a gap silently closed would compress
// a two-year lull into nothing.
//
// The `(unknown)` bucket never appears on the axis. A NULL issue date is not a point in time, and
// parking it at either end would draw a spike that means "no date" — the dashboards surface it beside
// the chart instead (the draft backlog, the no-expiry set).

import { scaleBand, scaleLinear } from "@visx/scale";
import { BarRounded } from "@visx/shape";
import { NoData } from "./ChartCard";
import { fmtInt, GEO, INK, MAGNITUDE, type Segment } from "./theme";

export function Histogram({
  segments,
  locale,
  unit,
  /** show at most this many tick labels; the rest are drawn without one */
  maxTicks = 14,
}: {
  segments: Segment[];
  locale: string;
  unit?: string;
  maxTicks?: number;
}) {
  if (segments.length === 0) return <NoData />;

  const { vbWidth, fontSize } = GEO;
  const plotHeight = 170;
  const labelBand = 58;
  const height = plotHeight + labelBand;
  const max = Math.max(...segments.map((s) => s.count), 1);
  const x = scaleBand<string>({
    domain: segments.map((s) => s.key),
    range: [0, vbWidth],
    padding: 0.08,
  });
  const y = scaleLinear<number>({ domain: [0, max], range: [plotHeight, 0] });
  const bw = x.bandwidth();
  const every = Math.max(1, Math.ceil(segments.length / maxTicks));

  return (
    <svg viewBox={`0 0 ${vbWidth} ${height}`} width="100%" height={height} role="img">
      {/* One recessive gridline at the maximum: enough to read the scale, not a grid. */}
      <line x1={0} x2={vbWidth} y1={y(max)} y2={y(max)} stroke={INK.grid} strokeWidth={1} />
      <text x={2} y={y(max) - 4} fontSize={fontSize - 1} fill={INK.muted}>
        {fmtInt(max, locale)}
      </text>
      <line x1={0} x2={vbWidth} y1={plotHeight} y2={plotHeight} stroke={INK.grid} strokeWidth={1} />

      {segments.map((s, i) => {
        const bx = x(s.key) ?? 0;
        const h = Math.max(plotHeight - y(s.count), s.count > 0 ? 1.5 : 0);
        const body = (
          <g>
            <title>{`${s.label}: ${fmtInt(s.count, locale)}${unit ? ` ${unit}` : ""}`}</title>
            <BarRounded
              x={bx}
              y={plotHeight - h}
              width={bw}
              height={h}
              radius={Math.min(GEO.radius, bw / 2)}
              top
              fill={s.color ?? MAGNITUDE}
            />
          </g>
        );
        return (
          <g key={s.key}>
            {s.href ? (
              <a href={s.href} className="chart-mark">
                {body}
              </a>
            ) : (
              body
            )}
            {i % every === 0 ? (
              <text
                x={bx + bw / 2}
                y={plotHeight + 10}
                textAnchor="end"
                fontSize={fontSize - 1}
                fill={INK.muted}
                transform={`rotate(-40 ${bx + bw / 2} ${plotHeight + 10})`}
              >
                {s.label}
              </text>
            ) : null}
          </g>
        );
      })}
    </svg>
  );
}
