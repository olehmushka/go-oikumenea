import Link from "next/link";
import { notFound } from "next/navigation";
import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader, Pager } from "@/components/ui";
import { DataTable } from "@/components/ontology/DataTable";
import { TypeBadge } from "@/components/ontology/TypeBadge";
import { OBJECT_TYPES, type Row } from "@/lib/ontology/registry";

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
  searchParams: Promise<{ pageToken?: string }>;
}) {
  const { type } = await params;
  const def = OBJECT_TYPES[type];
  if (!def || !def.list) notFound();

  const { pageToken } = await searchParams;
  let rows: Row[] = [];
  let nextPageToken: string | undefined;
  let error: unknown = null;
  try {
    const base = def.list.search ?? "";
    const search = pageToken
      ? `${base}${base ? "&" : "?"}pageToken=${encodeURIComponent(pageToken)}`
      : base;
    const res = await apiGet(def.list.path, search);
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
        title={def.labelPlural}
        description={def.blurb}
        action={
          <div className="flex items-center gap-3">
            <TypeBadge type={def.type} />
            {newRoute ? (
              <Link href={newRoute} className="btn-primary">
                New {def.label.toLowerCase()}
              </Link>
            ) : null}
          </div>
        }
      />
      {error ? <ErrorNotice error={error} /> : null}
      {!error && rows.length === 0 ? (
        <EmptyState>No {def.labelPlural.toLowerCase()} yet.</EmptyState>
      ) : null}
      {!error && rows.length > 0 ? (
        <>
          <DataTable type={type} rows={rows} />
          <Pager basePath={`/explore/${type}`} nextPageToken={nextPageToken} />
        </>
      ) : null}
    </div>
  );
}
