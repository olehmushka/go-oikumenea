"use client";

import { useCallback, useEffect, useState } from "react";
import { bffGet } from "@/lib/api/browser";
import { mutate } from "@/lib/api/client";
import { Table, Pill, Mono, EmptyState } from "@/components/ui";
import { T } from "@/components/T";
import type { ImportSource, ImportRun, WorkerJob, JobRef } from "@/lib/api/types";

type Tone = "green" | "red" | "indigo" | "amber" | "slate";

function tone(status: string): Tone {
  switch (status) {
    case "succeeded":
      return "green";
    case "failed":
    case "dead":
      return "red";
    case "running":
      return "indigo";
    case "queued":
      return "amber";
    default:
      return "slate";
  }
}

// The sources this screen surfaces first (countries + the two language schemes). Everything else
// (e.g. the per-country WOF geo-places sources) sorts after them, alphabetically.
const RELEVANT = new Set([
  "geo-countries-iso3166",
  "glottolog-languoids",
  "cldr-language-scripts",
]);

export function ImportsClient({
  initialSources,
  initialRuns,
  initialJobs,
}: {
  initialSources: ImportSource[];
  initialRuns: ImportRun[];
  initialJobs: WorkerJob[];
}) {
  const [runs, setRuns] = useState<ImportRun[]>(initialRuns);
  const [jobs, setJobs] = useState<WorkerJob[]>(initialJobs);
  const [busy, setBusy] = useState<string | null>(null);
  const [trigErr, setTrigErr] = useState<string | null>(null);
  const [pollErr, setPollErr] = useState<string | null>(null);

  const active = jobs.some((j) => j.status === "queued" || j.status === "running");

  const refresh = useCallback(async () => {
    try {
      const [r, j] = await Promise.all([
        bffGet<ImportRun[]>("/hermenea/v1/runs"),
        bffGet<WorkerJob[]>("/hermenea/v1/jobs"),
      ]);
      setRuns(r);
      setJobs(j);
      setPollErr(null);
    } catch (e) {
      setPollErr(e instanceof Error ? e.message : "refresh failed");
    }
  }, []);

  // Poll live while anything is in flight (1.5s), idle-poll otherwise (6s) so manually-triggered or
  // cron-enqueued jobs still surface without a reload.
  useEffect(() => {
    const id = setInterval(refresh, active ? 1500 : 6000);
    return () => clearInterval(id);
  }, [active, refresh]);

  async function trigger(code: string) {
    setBusy(code);
    setTrigErr(null);
    try {
      await mutate<JobRef>("POST", `/hermenea/v1/sync/${encodeURIComponent(code)}`);
      await refresh();
    } catch (e) {
      const b = e as { errorName?: string };
      setTrigErr(b?.errorName ?? (e instanceof Error ? e.message : "Trigger failed"));
    } finally {
      setBusy(null);
    }
  }

  // Latest job per source (the list is most-recent-first) → drives the per-source status + disabling.
  const latestJob = new Map<string, WorkerJob>();
  for (const j of jobs) {
    if (j.sourceCode && !latestJob.has(j.sourceCode)) latestJob.set(j.sourceCode, j);
  }

  const sources = [...initialSources].sort((a, b) => {
    const ra = RELEVANT.has(a.code) ? 0 : 1;
    const rb = RELEVANT.has(b.code) ? 0 : 1;
    return ra !== rb ? ra - rb : a.code.localeCompare(b.code);
  });

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-3 text-xs text-slate-500">
        <span className="inline-flex items-center gap-1.5">
          <span
            className={`h-2 w-2 rounded-full ${active ? "animate-pulse bg-indigo-500" : "bg-slate-300"}`}
          />
          {active ? <T>Live — jobs in flight</T> : <T>Auto-refreshing</T>}
        </span>
        <button type="button" className="btn-ghost" onClick={() => refresh()}>
          <T>Refresh now</T>
        </button>
        {pollErr ? <span className="text-red-500">{pollErr}</span> : null}
      </div>

      {trigErr ? (
        <div className="card border-red-200 bg-red-50 p-3 text-sm text-red-700">{trigErr}</div>
      ) : null}

      {/* ── Sources ─────────────────────────────────────────────────────────── */}
      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700">
          <T>Sources</T>
        </h2>
        {sources.length === 0 ? (
          <EmptyState>
            <T>No import sources. Is the hermenea proxy configured and reachable?</T>
          </EmptyState>
        ) : (
          <Table
            head={
              <>
                <th className="th"><T>Source</T></th>
                <th className="th"><T>Object type</T></th>
                <th className="th"><T>Connector</T></th>
                <th className="th"><T>Latest</T></th>
                <th className="th text-right"><T>Action</T></th>
              </>
            }
          >
            {sources.map((s) => {
              const job = latestJob.get(s.code);
              const inFlight =
                busy === s.code || job?.status === "queued" || job?.status === "running";
              return (
                <tr key={s.code} className={RELEVANT.has(s.code) ? "bg-indigo-50/40" : undefined}>
                  <td className="td">
                    <div className="font-medium text-slate-800">{s.name}</div>
                    <Mono>{s.code}</Mono>
                  </td>
                  <td className="td"><Mono>{s.objectType}</Mono></td>
                  <td className="td text-xs text-slate-500">
                    {s.connectorType}
                    {s.enabled ? "" : " · disabled"}
                  </td>
                  <td className="td">
                    {job ? <Pill tone={tone(job.status)}>{job.status}</Pill> : <span className="text-slate-400">—</span>}
                  </td>
                  <td className="td text-right">
                    <button
                      type="button"
                      disabled={inFlight}
                      onClick={() => trigger(s.code)}
                      className="rounded-md bg-indigo-600 px-3 py-1 text-xs font-medium text-white hover:bg-indigo-500 disabled:opacity-50"
                    >
                      {busy === s.code ? "…" : <T>Sync</T>}
                    </button>
                  </td>
                </tr>
              );
            })}
          </Table>
        )}
      </section>

      {/* ── Runs ────────────────────────────────────────────────────────────── */}
      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700">
          <T>Recent runs</T>
        </h2>
        {runs.length === 0 ? (
          <EmptyState><T>No import runs yet.</T></EmptyState>
        ) : (
          <Table
            head={
              <>
                <th className="th"><T>Source</T></th>
                <th className="th"><T>Status</T></th>
                <th className="th"><T>Created</T></th>
                <th className="th"><T>Updated</T></th>
                <th className="th"><T>Skipped</T></th>
                <th className="th"><T>Finished</T></th>
              </>
            }
          >
            {runs.map((r) => (
              <tr key={r.id}>
                <td className="td">
                  <Mono>{r.sourceCode}</Mono>
                  {r.sourceVersion ? <span className="ml-1 text-xs text-slate-400">{r.sourceVersion}</span> : null}
                  {r.error ? <div className="mt-1 text-xs text-red-600">{r.error}</div> : null}
                </td>
                <td className="td"><Pill tone={tone(r.status)}>{r.status}</Pill></td>
                <td className="td tabular-nums">{r.created}</td>
                <td className="td tabular-nums">{r.updated}</td>
                <td className="td tabular-nums">{r.skipped}</td>
                <td className="td whitespace-nowrap text-xs text-slate-500">{r.finishedAt ?? "—"}</td>
              </tr>
            ))}
          </Table>
        )}
      </section>

      {/* ── Jobs ────────────────────────────────────────────────────────────── */}
      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700">
          <T>Job queue</T>
        </h2>
        {jobs.length === 0 ? (
          <EmptyState><T>No jobs.</T></EmptyState>
        ) : (
          <Table
            head={
              <>
                <th className="th"><T>Source</T></th>
                <th className="th"><T>Status</T></th>
                <th className="th"><T>Attempts</T></th>
                <th className="th"><T>Error</T></th>
              </>
            }
          >
            {jobs.map((j) => (
              <tr key={j.id}>
                <td className="td"><Mono>{j.sourceCode ?? j.jobType}</Mono></td>
                <td className="td"><Pill tone={tone(j.status)}>{j.status}</Pill></td>
                <td className="td tabular-nums text-xs text-slate-500">
                  {j.attempts}/{j.maxAttempts}
                </td>
                <td className="td text-xs text-red-600">{j.lastError ?? ""}</td>
              </tr>
            ))}
          </Table>
        )}
      </section>
    </div>
  );
}
