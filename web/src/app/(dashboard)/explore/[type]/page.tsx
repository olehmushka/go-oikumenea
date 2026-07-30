import Link from "next/link";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import { oikumenea } from "@/lib/api/server";
import { capabilities } from "@/lib/api/capabilities";
import { EmptyState, ErrorNotice, PageHeader, Pager } from "@/components/ui";
import { DataTable } from "@/components/ontology/DataTable";
import { Dashboard } from "@/components/ontology/Dashboard";
import { FilterBar } from "@/components/ontology/FilterBar";
import { UnitTree } from "@/components/ontology/UnitTree";
import { TypeBadge } from "@/components/ontology/TypeBadge";
import { ViewToggle, type ViewKey } from "@/components/ontology/ViewToggle";
import { OBJECT_TYPES, type Row } from "@/lib/ontology/registry";
import {
  apiQuery,
  exploreExtraQuery,
  hasActiveFilters,
  readQuery,
  requiredFiltersSatisfied,
  toSearchParams,
} from "@/lib/ontology/filters";
import { T } from "@/components/T";
import { UnitCreateMenu } from "@/components/UnitCreateMenu";

// For org-scoped lists (units; D-TenantOrganizations, M40), fetch the organizations to populate the
// picker. Returns {id, label} options; label prefers the stable code, falling back to the RID tail.
async function loadOrgOptions(): Promise<{ id: string; label: string }[]> {
  const res = await oikumenea().then((ok) =>
    ok.request("GET", "/tenant/v1/organizations", { query: "?pageSize=200" }),
  );
  const orgs = ((res as { organizations?: unknown[] })?.organizations ?? []) as Record<string, unknown>[];
  return orgs
    .map((o) => ({ id: String(o.id ?? ""), label: String(o.code ?? o.id ?? "").trim() }))
    .filter((o) => o.id);
}

// Generic create routes for the few types with a bespoke create wizard; others create inline elsewhere.
const NEW_ROUTE: Record<string, string> = {
  person: "/persons/new",
  unit: "/units/new",
};

export default async function ExplorePage({
  params,
  searchParams,
}: {
  params: Promise<{ type: string }>;
  // Open-ended: the filter params are per-type (see def.filters), so a closed literal here would
  // make `?sex=male` unrepresentable. Only declared params are ever forwarded (lib/ontology/filters).
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { type } = await params;
  const def = OBJECT_TYPES[type];
  if (!def || !def.list) notFound();

  const sp = toSearchParams(await searchParams);
  const view = sp.get("view") ?? undefined;
  // Units support a hierarchical (expand-on-click) Tree view alongside the flat Table; any type with
  // a registry dashboard also has a chart view over the same filters (M57, D-ConsoleDashboards).
  //
  // The DASHBOARD is the default view wherever a type has one: opening a collection should answer
  // "what is in here" before it answers "what is on page 1", and the aggregate is the only view that
  // describes the WHOLE filtered set — a keyset page describes 50 rows. `?view=table` is the explicit
  // opt-out, and it is what every link into a row list carries. A type with no dashboard is
  // unaffected: `defaultView` is then `table` and no URL changes meaning.
  const hasDashboard = def.dashboard != null;
  const defaultView: ViewKey = hasDashboard ? "dashboard" : "table";
  const treeView = type === "unit" && view === "tree";
  const dashboardView = hasDashboard && !treeView && (view === undefined || view === "dashboard");
  const tableView = !treeView && !dashboardView;
  const views: ViewKey[] = [
    ...(hasDashboard ? (["dashboard"] as ViewKey[]) : []),
    "table",
    ...(type === "unit" ? (["tree"] as ViewKey[]) : []),
  ];
  // Backend substring search (persons, languoids): the query param narrows server-side; browsing
  // beyond the first page is via keyset pagination (pageToken/nextPageToken, the Pager below).
  const query = readQuery(def, sp);

  // Org-scoped lists (units; M40) require an `org` RID before the backend will list anything; without
  // one it rejects with Tenant:UnitInvalid. Load the picker options and only query once an org is
  // chosen — generalized to any `required` filter, of which `org` is the only one today.
  const orgScoped = def.list.orgScoped === true;
  const ready = requiredFiltersSatisfied(def, sp);
  let orgOptions: { id: string; label: string }[] = [];
  let error: unknown = null;
  if (orgScoped) {
    try {
      orgOptions = await loadOrgOptions();
    } catch (e) {
      error = e;
    }
  }

  const caps = await capabilities();

  let rows: Row[] = [];
  let nextPageToken: string | undefined;
  if (!error && ready && tableView) {
    try {
      const res = await oikumenea().then((ok) =>
        ok.request("GET", def.list!.path, { query: apiQuery(def, sp) }),
      );
      const parsed = def.list.parse(res);
      rows = parsed.rows;
      nextPageToken = parsed.nextPageToken;
    } catch (e) {
      error = e;
    }
  }

  const newRoute = NEW_ROUTE[type];
  const filtered = hasActiveFilters(def, sp);

  return (
    <div>
      <PageHeader
        title={<T>{def.labelPlural}</T>}
        description={def.blurb}
        action={
          <div className="flex items-center gap-3">
            <TypeBadge type={def.type} />
            {type === "unit" ? (
              // Units are created per-domain (military / university / …), not via one generic form.
              <UnitCreateMenu />
            ) : newRoute ? (
              <Link href={newRoute} className="btn-primary">
                <T>New</T> <span className="lowercase"><T>{def.label}</T></span>
              </Link>
            ) : null}
          </div>
        }
      />
      {/* useSearchParams needs a Suspense boundary; this route is dynamic, but the boundary costs
          nothing and keeps the build independent of that. */}
      <Suspense fallback={null}>
        <FilterBar type={type} caps={caps} orgOptions={orgOptions} />
      </Suspense>
      {ready ? (
        <ViewToggle
          type={type}
          sp={sp}
          views={views}
          current={treeView ? "tree" : dashboardView ? "dashboard" : "table"}
          defaultView={defaultView}
        />
      ) : null}
      {treeView && ready ? <UnitTree orgId={sp.get("org") ?? ""} /> : null}
      {error ? <ErrorNotice error={error} /> : null}
      {!error && dashboardView && ready ? (
        // A root-reach aggregate can take seconds; the boundary lets the header, the filter bar and
        // the view switch paint first rather than holding the whole page on the slowest count.
        <Suspense fallback={<EmptyState><T>Counting…</T></EmptyState>}>
          <Dashboard type={type} search={sp.toString()} />
        </Suspense>
      ) : null}
      {!error && !ready ? (
        <EmptyState><T>Select an organization to view its units.</T></EmptyState>
      ) : null}
      {!error && tableView && ready && rows.length === 0 ? (
        <EmptyState>
          {filtered ? (
            <T>No matches for these filters.</T>
          ) : query ? (
            <T>No matches.</T>
          ) : (
            <T>Nothing here yet.</T>
          )}
        </EmptyState>
      ) : null}
      {!error && tableView && rows.length > 0 ? (
        <>
          <DataTable type={type} rows={rows} />
          <Pager
            basePath={`/explore/${type}`}
            nextPageToken={nextPageToken}
            extraQuery={exploreExtraQuery(sp)}
          />
        </>
      ) : null}
    </div>
  );
}
