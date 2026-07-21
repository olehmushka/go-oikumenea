import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { can } from "@/lib/api/can";
import {
  EmptyState,
  ErrorNotice,
  Mono,
  PageHeader,
  Pill,
  Table,
} from "@/components/ui";
import { T } from "@/components/T";
import type {
  Connector,
  ConnectorPage,
  ConnectorSourceList,
  SyncRunPage,
} from "@/lib/api/types";

// Live operational state (last-seen, run status), never a cacheable view.
export const dynamic = "force-dynamic";

/** running → amber, succeeded → green, failed → red; anything else stays neutral. */
function runTone(state: string): "amber" | "green" | "red" | "slate" {
  if (state === "succeeded") return "green";
  if (state === "failed") return "red";
  if (state === "running") return "amber";
  return "slate";
}

/**
 * The connector-plane fleet view (M53 / D-ConnectorPlane) — READ-ONLY. It shows the family of
 * connectors that feed oikumenea (hermenea is the first), the sources each one has reported, and their
 * recent sync runs. There is deliberately **no** trigger/pause/retry control: the core has visibility,
 * not orchestration — scheduling lives inside the connector. `connector.read` is instance-scope and
 * sits in no base role, so in practice only instance admins reach this page (the endpoints re-decide
 * server-side regardless; can() only hides what would 403 — see lib/api/can.ts).
 */
export default async function ConnectorsPage({
  searchParams,
}: {
  searchParams: Promise<{ connectorId?: string; sourceId?: string }>;
}) {
  const { connectorId, sourceId } = await searchParams;

  // Gate the deep link too, not just the nav entry (display-only; the API is the authority).
  if (!(await can("connector.read"))) {
    return (
      <div>
        <PageHeader title={<T>Connectors</T>} />
        <EmptyState>
          <T>This surface is limited to instance administrators (connector.read).</T>
        </EmptyState>
      </div>
    );
  }

  let connectors: ConnectorPage | null = null;
  let sources: ConnectorSourceList | null = null;
  let runs: SyncRunPage | null = null;
  let error: unknown = null;
  let sourcesError: unknown = null;
  let runsError: unknown = null;

  try {
    connectors = await oikumenea().then((ok) => ok.connector.listConnectors(100));
  } catch (e) {
    error = e;
  }
  // Sources render as child rows under a selected connector (there is no global source list).
  if (connectorId) {
    try {
      sources = await oikumenea().then((ok) => ok.connector.listConnectorSources(connectorId));
    } catch (e) {
      sourcesError = e;
    }
  }
  // Runs: the whole fleet's recent runs by default, or filtered to a picked source, newest first.
  try {
    runs = await oikumenea().then((ok) => ok.connector.listSyncRuns(sourceId ?? null, 50));
  } catch (e) {
    runsError = e;
  }

  const list: Connector[] = connectors?.connectors ?? [];
  const selected = list.find((c) => c.id === connectorId);
  const sourceById = new Map((sources?.sources ?? []).map((s) => [s.id, s]));

  return (
    <div>
      <PageHeader
        title={<T>Connectors</T>}
        description={
          <T>
            The fleet of connectors that feed oikumenea — deployable agents beside the core, each with
            its own storage and scheduler, coupled over HTTP only. This is a read-only view: the core
            records what a connector reports and never triggers, pauses or retries a run. Scheduling
            lives inside the connector.
          </T>
        }
      />
      {error ? <ErrorNotice error={error} /> : null}

      {/* Connectors */}
      <h2 className="mb-3 text-sm font-semibold text-slate-900">
        <T>Fleet</T>
      </h2>
      {list.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Code</T></th>
              <th className="th"><T>Name</T></th>
              <th className="th"><T>Principal</T></th>
              <th className="th"><T>Status</T></th>
              <th className="th"><T>Last seen</T></th>
              <th className="th"></th>
            </>
          }
        >
          {list.map((c) => (
            <tr key={c.id} className={c.id === connectorId ? "bg-indigo-50/50" : undefined}>
              <td className="td">
                <Mono>{c.code}</Mono>
              </td>
              <td className="td">{c.name}</td>
              <td className="td">
                {c.principalId ? <Mono>{c.principalId.slice(-12)}</Mono> : "—"}
              </td>
              <td className="td">
                <Pill tone={c.status === "active" ? "green" : "slate"}>{c.status}</Pill>
              </td>
              <td className="td">{c.lastSeenAt ?? "—"}</td>
              <td className="td text-right">
                <Link
                  href={`/connectors?connectorId=${encodeURIComponent(c.id)}`}
                  className="text-xs font-medium text-indigo-600 hover:underline"
                >
                  <T>Sources</T>
                </Link>
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState>
          <T>No connectors have registered yet.</T>
        </EmptyState>
      )}

      {/* Sources of the selected connector */}
      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900">
        <T>Sources</T>
        {selected ? (
          <span className="ml-2 font-normal text-slate-500">
            — <Mono>{selected.code}</Mono>
          </span>
        ) : null}
      </h2>
      {sourcesError ? <ErrorNotice error={sourcesError} /> : null}
      {!connectorId ? (
        <EmptyState>
          <T>Pick a connector (Sources) to list the datasets it syncs.</T>
        </EmptyState>
      ) : sources && sources.sources.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Code</T></th>
              <th className="th"><T>Name</T></th>
              <th className="th"><T>Object type</T></th>
              <th className="th"><T>Schedule</T></th>
              <th className="th"><T>Enabled</T></th>
              <th className="th"></th>
            </>
          }
        >
          {sources.sources.map((s) => (
            <tr key={s.id} className={s.id === sourceId ? "bg-indigo-50/50" : undefined}>
              <td className="td">
                <Mono>{s.code}</Mono>
              </td>
              <td className="td">{s.name}</td>
              <td className="td">{s.objectType ? <Mono>{s.objectType}</Mono> : "—"}</td>
              <td className="td">{s.schedule ? <Mono>{s.schedule}</Mono> : "—"}</td>
              <td className="td">
                <Pill tone={s.enabled ? "green" : "slate"}>
                  {s.enabled ? <T>yes</T> : <T>no</T>}
                </Pill>
              </td>
              <td className="td text-right">
                <Link
                  href={`/connectors?connectorId=${encodeURIComponent(connectorId)}&sourceId=${encodeURIComponent(s.id)}`}
                  className="text-xs font-medium text-indigo-600 hover:underline"
                >
                  <T>Runs</T>
                </Link>
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState>
          <T>This connector has declared no sources.</T>
        </EmptyState>
      )}

      {/* Sync runs — fleet-wide, or filtered to a picked source */}
      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900">
        <T>Recent runs</T>
        {sourceId && sourceById.get(sourceId) ? (
          <span className="ml-2 font-normal text-slate-500">
            — <Mono>{sourceById.get(sourceId)!.code}</Mono>
          </span>
        ) : (
          <span className="ml-2 font-normal text-slate-500">
            — <T>whole fleet</T>
          </span>
        )}
        {sourceId ? (
          <Link
            href={connectorId ? `/connectors?connectorId=${encodeURIComponent(connectorId)}` : "/connectors"}
            className="ml-3 text-xs font-medium text-indigo-600 hover:underline"
          >
            <T>clear filter</T>
          </Link>
        ) : null}
      </h2>
      {runsError ? <ErrorNotice error={runsError} /> : null}
      {runs && runs.runs.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>State</T></th>
              <th className="th"><T>External run</T></th>
              <th className="th"><T>Created</T></th>
              <th className="th"><T>Updated</T></th>
              <th className="th"><T>Skipped</T></th>
              <th className="th"><T>Started</T></th>
              <th className="th"><T>Finished</T></th>
            </>
          }
        >
          {runs.runs.map((r) => (
            <tr key={r.id}>
              <td className="td">
                <Pill tone={runTone(r.state)}>{r.state}</Pill>
                {r.error ? (
                  <span className="ml-2 text-xs text-red-700" title={r.error}>
                    {r.error.length > 48 ? `${r.error.slice(0, 48)}…` : r.error}
                  </span>
                ) : null}
              </td>
              <td className="td">{r.externalRunId ? <Mono>{r.externalRunId}</Mono> : "—"}</td>
              <td className="td">{r.created}</td>
              <td className="td">{r.updated}</td>
              <td className="td">{r.skipped}</td>
              <td className="td">{r.startedAt}</td>
              <td className="td">{r.finishedAt ?? "—"}</td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState>
          {sourceId ? (
            <T>No runs reported for this source yet.</T>
          ) : (
            <T>No runs reported yet.</T>
          )}
        </EmptyState>
      )}
    </div>
  );
}
