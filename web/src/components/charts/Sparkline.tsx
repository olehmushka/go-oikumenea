// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// The shape of a series, small enough to sit inside a stat tile (M57 ticket 3). It answers "and which
// way is it going" beside a number that answers "how many" — the document dashboard's expiring-soon
// tile carries the next twelve months' expiry curve, so a flat 90-day count is not read as the whole
// story.
//
// Deliberately axis-less and label-less: a sparkline is a shape, not a chart. When the exact values
// matter, the Histogram beside it is the chart to read — this one carries no click target for the
// same reason.

import { scaleLinear } from "@visx/scale";
import { LinePath } from "@visx/shape";
import { INK, MAGNITUDE } from "./theme";

export function Sparkline({
  values,
  width = 132,
  height = 28,
  color = MAGNITUDE,
  title,
}: {
  values: number[];
  width?: number;
  height?: number;
  color?: string;
  title?: string;
}) {
  if (values.length < 2) return null;
  const x = scaleLinear<number>({ domain: [0, values.length - 1], range: [1, width - 1] });
  const y = scaleLinear<number>({
    domain: [0, Math.max(...values, 1)],
    range: [height - 2, 2],
  });
  const points = values.map((v, i) => ({ i, v }));

  return (
    <svg viewBox={`0 0 ${width} ${height}`} width={width} height={height} role="img" className="mt-2">
      {title ? <title>{title}</title> : null}
      <line
        x1={0}
        x2={width}
        y1={height - 1}
        y2={height - 1}
        stroke={INK.grid}
        strokeWidth={1}
      />
      <LinePath
        data={points}
        x={(d) => x(d.i)}
        y={(d) => y(d.v)}
        stroke={color}
        strokeWidth={2}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
