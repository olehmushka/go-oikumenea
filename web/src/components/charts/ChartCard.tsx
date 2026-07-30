// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// The frame every chart sits in (M57 ticket 3): a card, a title, an optional one-line note for the
// SQL semantic an operator would otherwise reverse-engineer from a surprising count, and an optional
// footer for the honest caveats (a top-15 cutoff, a bucket that is not clickable).
//
// The title is a <T> island rather than a server-resolved string: a chart title is UI chrome, and the
// explore page's RSC payload can be served from the router cache after a locale switch, which would
// leave a baked-in translation stale. Everything INSIDE the SVG is server-resolved instead — see the
// note in Dashboard.tsx.

import { T } from "@/components/T";

export function ChartCard({
  title,
  note,
  footer,
  wide = false,
  children,
}: {
  title: string;
  note?: string;
  footer?: React.ReactNode;
  /** span both columns of the dashboard grid (tile rows, pyramids, long histograms) */
  wide?: boolean;
  children: React.ReactNode;
}) {
  return (
    <section className={`card p-5 ${wide ? "lg:col-span-2" : ""}`}>
      <header className="mb-3">
        <h3 className="text-sm font-semibold text-slate-900">
          <T>{title}</T>
        </h3>
        {note ? (
          <p className="mt-0.5 text-xs text-slate-500">
            <T>{note}</T>
          </p>
        ) : null}
      </header>
      {children}
      {footer ? <div className="mt-2 text-xs text-slate-400">{footer}</div> : null}
    </section>
  );
}

/** The empty state of a chart whose facet came back with nothing to draw. */
export function NoData() {
  return (
    <p className="py-6 text-center text-sm text-slate-400">
      <T>Nothing to chart under these filters.</T>
    </p>
  );
}
