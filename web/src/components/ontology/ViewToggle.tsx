// The explorer's view switch (M57 ticket 3): Table · Tree · Dashboard.
//
// Generalizes the hand-built unit-only Table/Tree toggle. It is built on `exploreHref`, which rebuilds
// the WHOLE query string and always drops `pageToken`, so "the filter set survives the toggle" and "a
// view change returns to page 1" are properties of the URL builder rather than discipline here — the
// old hand-concatenated href silently dropped every filter but `org`.
//
// A Server Component: each view is a plain link, which is also why the filter set survives a shared
// link and the browser Back button.

import Link from "next/link";
import { T } from "@/components/T";
import { dashboardDefaults, exploreHref } from "@/lib/ontology/filters";
import { OBJECT_TYPES } from "@/lib/ontology/registry";

export type ViewKey = "table" | "tree" | "dashboard";

export function ViewToggle({
  type,
  sp,
  views,
  current,
  defaultView = "table",
}: {
  type: string;
  sp: URLSearchParams;
  /** in display order, the default first */
  views: ViewKey[];
  current: ViewKey;
  /**
   * The view a bare `/explore/<type>` shows — `dashboard` wherever the type has one. It is the ONE
   * view that clears `view` rather than setting it, so the canonical URL of a collection stays
   * paramless and a shared link never carries a redundant `?view=`.
   */
  defaultView?: ViewKey;
}) {
  if (views.length < 2) return null;
  // The dashboard link carries the type's default window (audit's last 30 days) when the URL sets
  // none of it — so arriving at the dashboard is already scoped, and the chip says so.
  const defaults = dashboardDefaults(OBJECT_TYPES[type], sp, new Date());
  const labels: Record<ViewKey, string> = {
    table: "Table",
    tree: "Tree",
    dashboard: "Dashboard",
  };
  return (
    <div className="mb-4 flex items-center gap-1 text-sm">
      {views.map((v) => (
        <Link
          key={v}
          href={exploreHref(type, sp, {
            view: v === defaultView ? undefined : v,
            ...(v === "dashboard" ? defaults : {}),
          })}
          className={v === current ? "btn-primary" : "btn-ghost"}
          aria-current={v === current ? "page" : undefined}
        >
          <T>{labels[v]}</T>
        </Link>
      ))}
    </div>
  );
}
