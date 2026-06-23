import Link from "next/link";
import { notFound } from "next/navigation";
import { oikumenea } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader, Pager } from "@/components/ui";
import { DataTable } from "@/components/ontology/DataTable";
import { TypeBadge } from "@/components/ontology/TypeBadge";
import { OBJECT_TYPES, type Row } from "@/lib/ontology/registry";
import { T } from "@/components/T";

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
  searchParams: Promise<{ pageToken?: string; q?: string }>;
}) {
  const { type } = await params;
  const def = OBJECT_TYPES[type];
  if (!def || !def.list) notFound();

  const { pageToken, q } = await searchParams;
  // Backend substring search (e.g. languoids): the catalog is large and the list isn't paginable past
  // its limit, so the query param is the only way to reach rows beyond the first page.
  const query = def.list.searchParam ? (q ?? "").trim() : "";
  let rows: Row[] = [];
  let nextPageToken: string | undefined;
  let error: unknown = null;
  try {
    let search = def.list.search ?? "";
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

  const newRoute = NEW_ROUTE[type];

  return (
    <div>
      <PageHeader
        title={<T>{def.labelPlural}</T>}
        description={def.blurb}
        action={
          <div className="flex items-center gap-3">
            <TypeBadge type={def.type} />
            {newRoute ? (
              <Link href={newRoute} className="btn-primary">
                <T>New</T> <span className="lowercase"><T>{def.label}</T></span>
              </Link>
            ) : null}
          </div>
        }
      />
      {def.list.searchParam ? (
        <form method="get" action={`/explore/${type}`} className="mb-4 flex items-center gap-2">
          <input
            type="search"
            name="q"
            defaultValue={query}
            placeholder="Search by name or code…"
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
      {!error && rows.length === 0 ? (
        <EmptyState>
          {query ? <T>No matches.</T> : <T>Nothing here yet.</T>}
        </EmptyState>
      ) : null}
      {!error && rows.length > 0 ? (
        <>
          <DataTable type={type} rows={rows} />
          <Pager
            basePath={`/explore/${type}`}
            nextPageToken={nextPageToken}
            extraQuery={query ? `q=${encodeURIComponent(query)}` : ""}
          />
        </>
      ) : null}
    </div>
  );
}
