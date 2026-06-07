import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import {
  Card,
  ErrorNotice,
  EmptyState,
  Mono,
  Pager,
  PageHeader,
  Pill,
  Table,
} from "@/components/ui";
import { Localized } from "@/components/Localized";
import { GraphManager } from "@/components/GraphManager";
import type { GraphList, UnitPage } from "@/lib/api/types";

export default async function UnitsPage({
  searchParams,
}: {
  searchParams: Promise<{ pageToken?: string }>;
}) {
  const { pageToken } = await searchParams;
  let page: UnitPage | null = null;
  let graphs: GraphList | null = null;
  let error: unknown = null;
  try {
    const qs = new URLSearchParams({ pageSize: "50" });
    if (pageToken) qs.set("pageToken", pageToken);
    [page, graphs] = await Promise.all([
      apiGet<UnitPage>("/tenant/v1/units", `?${qs}`),
      apiGet<GraphList>("/tenant/v1/graphs").catch(() => null),
    ]);
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title="Units"
        description="The organization as units across named hierarchies (a DAG, not a tree)."
        action={
          <Link href="/units/new" className="btn-primary">
            New unit
          </Link>
        }
      />
      {error ? <ErrorNotice error={error} /> : null}

      <Card className="mb-4">
        <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">Graphs</div>
        <div className="mt-2">
          <GraphManager graphs={graphs?.graphs ?? []} />
        </div>
      </Card>

      {page && page.units.length === 0 && <EmptyState>No units yet.</EmptyState>}

      {page && page.units.length > 0 && (
        <>
          <Table
            head={
              <>
                <th className="th">Code</th>
                <th className="th">Name</th>
                <th className="th">Kind</th>
                <th className="th">Level</th>
                <th className="th">Visibility</th>
                <th className="th">State</th>
              </>
            }
          >
            {page.units.map((u) => (
              <tr key={u.id} className="hover:bg-slate-50">
                <td className="td">
                  <Link href={`/units/${u.id}`} className="text-indigo-600 hover:underline">
                    <Mono>{u.code}</Mono>
                  </Link>
                </td>
                <td className="td">
                  <Localized map={u.name} />
                </td>
                <td className="td">{u.unitKind ?? "—"}</td>
                <td className="td">{u.level ?? "—"}</td>
                <td className="td">
                  <Pill tone={u.visibility === "SHADOW" ? "amber" : "green"}>
                    {u.visibility ?? "—"}
                  </Pill>
                </td>
                <td className="td">
                  <Pill tone={u.state === "ACTIVE" ? "green" : "slate"}>
                    {u.state ?? "—"}
                  </Pill>
                </td>
              </tr>
            ))}
          </Table>
          <Pager basePath="/units" nextPageToken={page.nextPageToken} />
        </>
      )}
    </div>
  );
}
