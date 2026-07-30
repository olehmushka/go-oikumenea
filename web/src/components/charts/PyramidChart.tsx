// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// The age pyramid (M57 ticket 3, facets.md ①) — the canonical personnel-structure view: age bands
// stacked youngest-at-the-bottom, two series mirrored about a centre axis carrying the band label.
//
// It is the ONE chart the stats contract cannot answer in a single call. A distribution is per-facet;
// this is a CROSS-TAB (birthdate × sex), and D-ObjectFacets deliberately has no cross-facet grouping
// (facets.md § Open seams). So the dashboard asks TWICE — the same stats endpoint, the same filter
// state, plus `sex=male` / `sex=female` and `facets=birthdate` — which is not a workaround but the
// design working: a wing is exactly the list the filter describes, so clicking a bar lands on it.
//
// The two wings share ONE x scale. Scaling each wing to its own maximum would make an 80/20 split
// look symmetric, which is the single thing this chart exists to show.
//
// Only the two binary ISO-5218 values are drawn. `not_known` and `not_applicable` are real values in
// this directory and are NOT silently dropped: the dashboard reports them beside the chart, where a
// data-quality number belongs, rather than inventing a third wing.

import { scaleLinear } from "@visx/scale";
import { BarRounded } from "@visx/shape";
import { NoData } from "./ChartCard";
import { clip, fmtInt, GEO, INK, SERIES, SYNTHETIC_FILL, type Segment } from "./theme";

export type PyramidRow = {
  /** the shared band key, e.g. "25-34" — also the row label */
  key: string;
  label: string;
  left: Segment;
  right: Segment;
};

export function PyramidChart({
  rows,
  leftLabel,
  rightLabel,
  locale,
}: {
  rows: PyramidRow[];
  leftLabel: string;
  rightLabel: string;
  locale: string;
}) {
  if (rows.length === 0) return <NoData />;

  const { vbWidth, rowHeight, barGap, radius, fontSize } = GEO;
  const centre = 74; // the band-label column, shared by both wings
  const wing = (vbWidth - centre) / 2 - GEO.valueGutter / 2;
  const height = rows.length * rowHeight + 22;
  const max = Math.max(...rows.flatMap((r) => [r.left.count, r.right.count]), 1);
  const x = scaleLinear<number>({ domain: [0, max], range: [0, wing] });
  const midL = vbWidth / 2 - centre / 2;
  const midR = vbWidth / 2 + centre / 2;

  const colour = (s: Segment, slot: number) =>
    s.color ?? (s.href ? SERIES[slot] : SYNTHETIC_FILL);

  return (
    <div>
      <div className="mb-1 flex items-center justify-center gap-6 text-xs text-slate-600">
        <span className="flex items-center gap-1.5">
          <span aria-hidden className="size-2.5 rounded-sm" style={{ backgroundColor: SERIES[0] }} />
          {leftLabel}
        </span>
        <span className="flex items-center gap-1.5">
          <span aria-hidden className="size-2.5 rounded-sm" style={{ backgroundColor: SERIES[1] }} />
          {rightLabel}
        </span>
      </div>
      <svg viewBox={`0 0 ${vbWidth} ${height}`} width="100%" height={height} role="img">
        {/* Oldest band on top, so the pyramid stands on its youngest cohort. */}
        {[...rows].reverse().map((r, i) => {
          const y = i * rowHeight + 2;
          const lw = Math.max(x(r.left.count), r.left.count > 0 ? 2 : 0);
          const rw = Math.max(x(r.right.count), r.right.count > 0 ? 2 : 0);
          const wingBar = (s: Segment, slot: number, xPos: number, w: number, side: "l" | "r") => {
            const body = (
              <g>
                <title>{`${s.label}: ${fmtInt(s.count, locale)}`}</title>
                <BarRounded
                  x={xPos}
                  y={y + barGap / 2}
                  width={w}
                  height={rowHeight - barGap}
                  radius={radius}
                  left={side === "l"}
                  right={side === "r"}
                  fill={colour(s, slot)}
                />
                <text
                  x={side === "l" ? xPos - 6 : xPos + w + 6}
                  y={y + rowHeight / 2}
                  textAnchor={side === "l" ? "end" : "start"}
                  dominantBaseline="middle"
                  fontSize={fontSize - 1}
                  fill={INK.secondary}
                >
                  {fmtInt(s.count, locale)}
                </text>
              </g>
            );
            return s.href ? (
              <a href={s.href} className="chart-mark">
                {body}
              </a>
            ) : (
              body
            );
          };
          return (
            <g key={r.key}>
              {wingBar(r.left, 0, midL - lw, lw, "l")}
              <text
                x={vbWidth / 2}
                y={y + rowHeight / 2}
                textAnchor="middle"
                dominantBaseline="middle"
                fontSize={fontSize}
                fill={INK.secondary}
              >
                {clip(r.label, 9)}
              </text>
              {wingBar(r.right, 1, midR, rw, "r")}
            </g>
          );
        })}
      </svg>
    </div>
  );
}
