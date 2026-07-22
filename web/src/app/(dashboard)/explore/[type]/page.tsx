import Link from "next/link";
import { notFound } from "next/navigation";
import { oikumenea } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader, Pager } from "@/components/ui";
import { DataTable } from "@/components/ontology/DataTable";
import { UnitTree } from "@/components/ontology/UnitTree";
import { TypeBadge } from "@/components/ontology/TypeBadge";
import { OBJECT_TYPES, type Row } from "@/lib/ontology/registry";
import { T } from "@/components/T";
import { UnitCreateMenu } from "@/components/UnitCreateMenu";
import { tg } from "@/lib/messages";

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
  searchParams: Promise<{ pageToken?: string; q?: string; org?: string; view?: string }>;
}) {
  const { type } = await params;
  const def = OBJECT_TYPES[type];
  if (!def || !def.list) notFound();

  const { pageToken, q, org, view } = await searchParams;
  // Units support a hierarchical (expand-on-click) Tree view alongside the flat Table.
  const treeView = type === "unit" && view === "tree";
  // Backend substring search (e.g. languoids): the catalog is large, so the query param narrows it;
  // browsing beyond the first page is via keyset pagination (pageToken/nextPageToken, the Pager below).
  const query = def.list.searchParam ? (q ?? "").trim() : "";

  // Org-scoped lists (units; M40) require an `org` RID before the backend will list anything; without
  // one it rejects with Tenant:UnitInvalid. Load the picker options and only query once an org is chosen.
  const orgScoped = def.list.orgScoped === true;
  const orgId = orgScoped ? (org ?? "").trim() : "";
  let orgOptions: { id: string; label: string }[] = [];
  let error: unknown = null;
  if (orgScoped) {
    try {
      orgOptions = await loadOrgOptions();
    } catch (e) {
      error = e;
    }
  }

  let rows: Row[] = [];
  let nextPageToken: string | undefined;
  if (!error && (!orgScoped || orgId) && !treeView) {
    try {
      let search = def.list.search ?? "";
      if (orgScoped && orgId) {
        search += `${search ? "&" : "?"}org=${encodeURIComponent(orgId)}`;
      }
      if (query) {
        search += `${search ? "&" : "?"}${def.list.searchParam}=${encodeURIComponent(query)}`;
      }
      if (pageToken) {
        search += `${search ? "&" : "?"}pageToken=${encodeURIComponent(pageToken)}`;
      }
      const listPath = def.list.path;
      const res = await oikumenea().then((ok) => ok.request("GET", listPath, { query: search }));
      const parsed = def.list.parse(res);
      rows = parsed.rows;
      nextPageToken = parsed.nextPageToken;
    } catch (e) {
      error = e;
    }
  }

  const newRoute = NEW_ROUTE[type];
  const pagerExtra = [query ? `q=${encodeURIComponent(query)}` : "", orgId ? `org=${encodeURIComponent(orgId)}` : ""]
    .filter(Boolean)
    .join("&");

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
      {orgScoped ? (
        <form method="get" action={`/explore/${type}`} className="mb-4 flex items-center gap-2">
          <label className="label mb-0"><T>Organization</T></label>
          <select name="org" defaultValue={orgId} className="input max-w-xs">
            <option value=""><T>Select an organization…</T></option>
            {orgOptions.map((o) => (
              <option key={o.id} value={o.id}>{o.label}</option>
            ))}
          </select>
          <button type="submit" className="btn-primary"><T>Show</T></button>
        </form>
      ) : null}
      {type === "unit" && orgId ? (
        <div className="mb-4 flex items-center gap-1 text-sm">
          <Link
            href={`/explore/unit?org=${encodeURIComponent(orgId)}`}
            className={treeView ? "btn-ghost" : "btn-primary"}
          >
            <T>Table</T>
          </Link>
          <Link
            href={`/explore/unit?org=${encodeURIComponent(orgId)}&view=tree`}
            className={treeView ? "btn-primary" : "btn-ghost"}
          >
            <T>Tree</T>
          </Link>
        </div>
      ) : null}
      {treeView && orgId ? <UnitTree orgId={orgId} /> : null}
      {!treeView && def.list.searchParam ? (
        <form method="get" action={`/explore/${type}`} className="mb-4 flex items-center gap-2">
          <input
            type="search"
            name="q"
            defaultValue={query}
            placeholder={tg("Search by name or code…")}
            className="input max-w-xs"
            autoComplete="off"
          />
          <button type="submit" className="btn-primary"><T>Search</T></button>
          {query ? (
            <Link href={`/explore/${type}`} className="btn-ghost"><T>Clear</T></Link>
          ) : null}
        </form>
      ) : null}
      {error ? <ErrorNotice error={error} /> : null}
      {!error && orgScoped && !orgId ? (
        <EmptyState><T>Select an organization to view its units.</T></EmptyState>
      ) : null}
      {!error && !treeView && (!orgScoped || orgId) && rows.length === 0 ? (
        <EmptyState>
          {query ? <T>No matches.</T> : <T>Nothing here yet.</T>}
        </EmptyState>
      ) : null}
      {!error && !treeView && rows.length > 0 ? (
        <>
          <DataTable type={type} rows={rows} />
          <Pager
            basePath={`/explore/${type}`}
            nextPageToken={nextPageToken}
            extraQuery={pagerExtra}
          />
        </>
      ) : null}
    </div>
  );
}
