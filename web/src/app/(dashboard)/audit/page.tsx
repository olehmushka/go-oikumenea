import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, Pager, PageHeader, Pill, Table } from "@/components/ui";
import { LookupForm } from "@/components/LookupForm";
import type { AuditEntryPage } from "@/lib/api/types";

export default async function AuditPage({
  searchParams,
}: {
  searchParams: Promise<{ action?: string; outcome?: string; pageToken?: string }>;
}) {
  const { action, outcome, pageToken } = await searchParams;
  let page: AuditEntryPage | null = null;
  let error: unknown = null;
  try {
    const qs = new URLSearchParams({ pageSize: "50" });
    if (action) qs.set("action", action);
    if (outcome) qs.set("outcome", outcome);
    if (pageToken) qs.set("pageToken", pageToken);
    page = await apiGet<AuditEntryPage>("/audit/v1/audit", `?${qs}`);
  } catch (e) {
    error = e;
  }

  const extra = [
    action ? `action=${encodeURIComponent(action)}` : "",
    outcome ? `outcome=${encodeURIComponent(outcome)}` : "",
  ]
    .filter(Boolean)
    .join("&");

  return (
    <div>
      <PageHeader
        title="Audit log"
        description="Append-only trail of permission-sensitive actions. Reads are themselves permission-scoped."
      />

      <div className="mb-5 max-w-md">
        <LookupForm
          basePath="/audit"
          param="action"
          label="Filter by action"
          placeholder="e.g. assignment.grant"
          current={action}
        />
      </div>

      {error ? <ErrorNotice error={error} /> : null}
      {page && (page.entries?.length ?? 0) === 0 && <EmptyState>No audit entries.</EmptyState>}
      {page && (page.entries?.length ?? 0) > 0 && (
        <>
          <Table
            head={
              <>
                <th className="th">Time</th>
                <th className="th">Action</th>
                <th className="th">Actor</th>
                <th className="th">Target</th>
                <th className="th">Outcome</th>
              </>
            }
          >
            {page.entries!.map((e) => (
              <tr key={e.id}>
                <td className="td whitespace-nowrap text-xs text-slate-500">
                  {e.createdAt ?? "—"}
                </td>
                <td className="td">
                  <Mono>{e.action}</Mono>
                </td>
                <td className="td">
                  {e.actorType === "SYSTEM" ? (
                    <Pill>system</Pill>
                  ) : (
                    <Mono>{e.actorPersonId?.slice(-8) ?? "—"}</Mono>
                  )}
                </td>
                <td className="td">
                  <span className="text-slate-500">{e.targetType ?? ""}</span>{" "}
                  {e.targetId ? <Mono>{e.targetId.slice(-8)}</Mono> : ""}
                </td>
                <td className="td">
                  <Pill
                    tone={
                      e.outcome === "SUCCESS"
                        ? "green"
                        : e.outcome === "DENIED"
                          ? "amber"
                          : "red"
                    }
                  >
                    {e.outcome ?? "—"}
                  </Pill>
                </td>
              </tr>
            ))}
          </Table>
          <Pager basePath="/audit" nextPageToken={page.nextPageToken} extraQuery={extra} />
        </>
      )}
    </div>
  );
}
