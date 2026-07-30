// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// A single number, big (M57 ticket 3). Sometimes the honest answer is not a chart: "how many active
// memberships", "the revocation rate", "documents expiring within 90 days" are one number each, and a
// two-bar chart of a two-value enum is a table with extra steps.
//
// A tile is a link wherever the number is expressible as a filter (a status tile narrows the list to
// that status), and inert where it is not (a derived ratio is not a row set). The tone is the
// console's status palette — the same red a revoked pill wears — never a categorical slot.

import { fmtInt, TONE_FILL, type Segment } from "./theme";
import type { Tone } from "@/lib/ontology/registry";

export function StatTile({
  label,
  value,
  sub,
  tone = "slate",
  href,
  children,
}: {
  label: string;
  /** already formatted: a count, a percentage, a ratio */
  value: string;
  sub?: string;
  tone?: Tone;
  href?: string;
  /** an optional trend under the number (Sparkline) */
  children?: React.ReactNode;
}) {
  const body = (
    <>
      <div className="flex items-baseline gap-2">
        <span
          aria-hidden
          className="size-2 shrink-0 rounded-full"
          style={{ backgroundColor: TONE_FILL[tone] }}
        />
        <span className="truncate text-xs font-medium uppercase tracking-wide text-slate-500">
          {label}
        </span>
      </div>
      <div className="mt-1 text-2xl font-semibold tabular-nums text-slate-900">{value}</div>
      {sub ? <div className="text-xs text-slate-400">{sub}</div> : null}
      {children}
    </>
  );
  const className = "card block p-4 transition-colors";
  return href ? (
    <a href={href} className={`${className} hover:border-indigo-300 hover:bg-indigo-50/40`}>
      {body}
    </a>
  ) : (
    <div className={className}>{body}</div>
  );
}

/** The tile row a dashboard leads with: the filtered total first, then the status split. */
export function StatTileRow({ children }: { children: React.ReactNode }) {
  return <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">{children}</div>;
}

/** Tiles straight off an enum distribution — the shape four of the five dashboards lead with. */
export function segmentTiles(
  segments: Segment[],
  locale: string,
  tones: Record<string, Tone> = {},
): React.ReactNode[] {
  return segments.map((s) => (
    <StatTile
      key={s.key}
      label={s.label}
      value={fmtInt(s.count, locale)}
      tone={tones[s.key] ?? "slate"}
      href={s.href}
    />
  ));
}
