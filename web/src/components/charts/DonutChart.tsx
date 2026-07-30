// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Composition of a whole (M57 ticket 3): the sex donut, the unit-kind mix, the document status split.
//
// Slices are IDENTITY, so they take the categorical slots in fixed order — and the order is the
// bucket order the API returns, which for an enum is the declared CHECK-set order with zero-count
// buckets included. That is deliberate: the shape of the chart stays stable across filterings, and a
// slice keeps its colour when a filter removes another one (colour follows the entity, never its
// rank). `(unknown)` and `(other)` are grey, not slots — see theme.ts.
//
// The legend is not optional and it carries the COUNT: three of the six categorical slots sit below
// 3:1 against white, so the palette's relief rule obliges a visible label per slice. It also makes
// the donut readable without colour at all, which a ring of arcs otherwise is not.
//
// Server Component: the legend rows are the same <a> as the arcs, so the click target is generous
// without any JS.

import { Pie } from "@visx/shape";
import { NoData } from "./ChartCard";
import { fmtInt, fmtPct, INK, seriesColor, SYNTHETIC_FILL, type Segment } from "./theme";

const SIZE = 168;

export function DonutChart({
  segments,
  locale,
  /** the count the slices are a share of — the totalCount of the filtered set */
  total,
}: {
  segments: Segment[];
  locale: string;
  total: number;
}) {
  const drawn = segments.filter((s) => s.count > 0);
  if (drawn.length === 0) return <NoData />;

  const color = (s: Segment, i: number): string =>
    s.color ?? (s.href ? seriesColor(i) : SYNTHETIC_FILL);
  const r = SIZE / 2;

  return (
    <div className="flex flex-wrap items-center gap-5">
      <svg viewBox={`0 0 ${SIZE} ${SIZE}`} width={SIZE} height={SIZE} role="img" className="shrink-0">
        <g transform={`translate(${r}, ${r})`}>
          <Pie
            data={drawn}
            pieValue={(s) => s.count}
            outerRadius={r - 1}
            innerRadius={(r - 1) * 0.62}
            padAngle={0.012}
            pieSort={null}
          >
            {(pie) =>
              pie.arcs.map((arc, i) => {
                const s = arc.data;
                const path = (
                  <>
                    <title>{`${s.label}: ${fmtInt(s.count, locale)} (${fmtPct(s.count, total, locale)})`}</title>
                    <path
                      d={pie.path(arc) ?? undefined}
                      fill={color(s, i)}
                      stroke={INK.surface}
                      strokeWidth={2}
                    />
                  </>
                );
                return s.href ? (
                  <a key={s.key} href={s.href} className="chart-mark">
                    {path}
                  </a>
                ) : (
                  <g key={s.key}>{path}</g>
                );
              })
            }
          </Pie>
          <text
            textAnchor="middle"
            dominantBaseline="middle"
            y={-2}
            fontSize={16}
            fontWeight={600}
            fill={INK.primary}
          >
            {fmtInt(total, locale)}
          </text>
        </g>
      </svg>

      <ul className="min-w-0 flex-1 space-y-1 text-sm">
        {drawn.map((s, i) => {
          const row = (
            <>
              <span
                aria-hidden
                className="inline-block size-2.5 shrink-0 rounded-sm"
                style={{ backgroundColor: color(s, i) }}
              />
              <span className="min-w-0 flex-1 truncate text-slate-700">{s.label}</span>
              <span className="tabular-nums text-slate-900">{fmtInt(s.count, locale)}</span>
              <span className="w-14 text-right tabular-nums text-slate-400">
                {fmtPct(s.count, total, locale)}
              </span>
            </>
          );
          return (
            <li key={s.key}>
              {s.href ? (
                <a href={s.href} className="flex items-center gap-2 rounded px-1 py-0.5 hover:bg-slate-50">
                  {row}
                </a>
              ) : (
                <span className="flex items-center gap-2 px-1 py-0.5">{row}</span>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
