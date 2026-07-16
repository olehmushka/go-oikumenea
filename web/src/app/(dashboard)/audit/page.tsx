import { oikumenea } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, Pager, PageHeader, Pill, Table } from "@/components/ui";
import { ActionFilter } from "@/components/ActionFilter";
import { ActionParamsList } from "@/components/ActionParamsList";
import { T } from "@/components/T";
import type { ActionType, AuditEntryPage } from "@/lib/api/types";

export default async function AuditPage({
  searchParams,
}: {
  searchParams: Promise<{ action?: string; outcome?: string; pageToken?: string }>;
}) {
  const { action, outcome, pageToken } = await searchParams;
  let page: AuditEntryPage | null = null;
  let actionTypes: ActionType[] = [];
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    [page, actionTypes] = await Promise.all([
      ok.audit.query(
        undefined, // actorPersonId
        undefined, // actorType
        undefined, // targetType
        undefined, // targetId
        undefined, // unitId
        action ?? undefined,
        (outcome ?? undefined) as never, // AuditOutcome enum; value is the wire string
        undefined, // since
        undefined, // until
        50,
        pageToken ?? undefined,
      ),
      ok.audit.listActionTypes().catch(() => [] as ActionType[]),
    ]);
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
        title={<T>Audit log</T>}
        description={<T>Append-only trail of permission-sensitive actions. Reads are themselves permission-scoped.</T>}
      />

      <div className="mb-5 max-w-md">
        {actionTypes.length > 0 ? (
          <ActionFilter current={action} actions={actionTypes} />
        ) : null}
      </div>

      {actionTypes.length > 0 ? (
        <details className="mb-5">
          <summary className="cursor-pointer text-sm font-medium text-indigo-600 hover:underline">
            <T>Action-type catalog</T> ({actionTypes.length})
          </summary>
          <p className="mb-3 mt-2 text-xs text-slate-500">
            <T>Every registered action code (D-ActionTypes, R-29): the machine catalog behind the free-text audit action, with its owning module and gating write permission.</T>
          </p>
          <Table
            head={
              <>
                <th className="th"><T>Code</T></th>
                <th className="th"><T>Service</T></th>
                <th className="th"><T>Target type</T></th>
                <th className="th"><T>Permission</T></th>
                <th className="th"><T>Parameters</T></th>
              </>
            }
          >
            {[...actionTypes]
              .sort((a, b) => a.code.localeCompare(b.code))
              .map((a) => (
                <tr key={a.code}>
                  <td className="td align-top"><Mono>{a.code}</Mono></td>
                  <td className="td align-top text-slate-600">{a.service}</td>
                  <td className="td align-top text-slate-600">{a.targetType}</td>
                  <td className="td align-top"><Mono>{a.permission}</Mono></td>
                  <td className="td align-top"><ActionParamsList params={a.parameters} /></td>
                </tr>
              ))}
          </Table>
        </details>
      ) : null}

      {error ? <ErrorNotice error={error} /> : null}
      {page && (page.entries?.length ?? 0) === 0 && <EmptyState><T>No audit entries.</T></EmptyState>}
      {page && (page.entries?.length ?? 0) > 0 && (
        <>
          <Table
            head={
              <>
                <th className="th"><T>Time</T></th>
                <th className="th"><T>Action</T></th>
                <th className="th"><T>Actor</T></th>
                <th className="th"><T>Target</T></th>
                <th className="th"><T>Outcome</T></th>
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
                    <Pill><T>system</T></Pill>
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
