import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import {
  EmptyState,
  ErrorNotice,
  Mono,
  Pager,
  PageHeader,
  Pill,
  Table,
} from "@/components/ui";
import type { PersonPage } from "@/lib/api/types";

export default async function PersonsPage({
  searchParams,
}: {
  searchParams: Promise<{ pageToken?: string }>;
}) {
  const { pageToken } = await searchParams;
  let page: PersonPage | null = null;
  let error: unknown = null;
  try {
    const qs = new URLSearchParams({ pageSize: "50" });
    if (pageToken) qs.set("pageToken", pageToken);
    page = await apiGet<PersonPage>("/person/v1/persons", `?${qs}`);
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title="Persons"
        description="The instance-global personnel directory. Account-optional."
        action={
          <Link href="/persons/new" className="btn-primary">
            New person
          </Link>
        }
      />
      {error ? <ErrorNotice error={error} /> : null}
      {page && page.persons.length === 0 && <EmptyState>No persons yet.</EmptyState>}
      {page && page.persons.length > 0 && (
        <>
          <Table
            head={
              <>
                <th className="th">Code</th>
                <th className="th">Display name</th>
                <th className="th">Sex</th>
                <th className="th">Birthdate</th>
                <th className="th">Status</th>
              </>
            }
          >
            {page.persons.map((p) => (
              <tr key={p.id} className="hover:bg-slate-50">
                <td className="td">
                  <Link
                    href={`/persons/${p.id}`}
                    className="text-indigo-600 hover:underline"
                  >
                    <Mono>{p.code || p.id.slice(-8)}</Mono>
                  </Link>
                </td>
                <td className="td font-medium text-slate-800">
                  {p.displayName ?? "—"}
                </td>
                <td className="td">{p.sex ?? "—"}</td>
                <td className="td">{p.birthdate ?? "—"}</td>
                <td className="td">
                  <Pill tone={p.status === "ACTIVE" ? "green" : "slate"}>
                    {p.status ?? "—"}
                  </Pill>
                </td>
              </tr>
            ))}
          </Table>
          <Pager basePath="/persons" nextPageToken={page.nextPageToken} />
        </>
      )}
    </div>
  );
}
