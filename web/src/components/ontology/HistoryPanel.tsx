"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { EmptyState, Mono, Pill } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import type { ObjectHistoryEvent } from "@/lib/api/types";

const PAGE = 25;

/** The reverse-chronological change history of one object, read from the audit ledger via
 *  getObjectHistory (D-Temporal tier b, R-31). Timeline (when/what/who) is visible under `audit.read`;
 *  the before/after change payloads are **redacted** (null) unless the caller holds the sensitive-reader
 *  capability — signalled by the `redacted` flag on the response. Reusable across /o/[rid], person, unit. */
export function HistoryPanel({ rid }: { rid: string }) {
  const [events, setEvents] = useState<ObjectHistoryEvent[]>([]);
  const [token, setToken] = useState<string | null | undefined>(undefined);
  const [redacted, setRedacted] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [open, setOpen] = useState<Set<string>>(new Set());

  const fetchPage = async (pageToken?: string | null) => {
    setBusy(true);
    setErr(null);
    try {
      const res = await api.audit.getObjectHistory(rid, PAGE, pageToken ?? undefined);
      setEvents((prev) => (pageToken ? [...prev, ...(res.events ?? [])] : res.events ?? []));
      setToken(res.nextPageToken);
      setRedacted(res.redacted);
      setLoaded(true);
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    setEvents([]);
    setLoaded(false);
    fetchPage();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rid]);

  if (err) return <ErrorBox error={err} />;
  if (!loaded && busy) return <p className="text-sm text-slate-400"><T>Loading history…</T></p>;
  if (loaded && events.length === 0)
    return <EmptyState><T>No recorded history for this object.</T></EmptyState>;

  return (
    <div>
      {redacted ? (
        <p className="mb-3 text-xs text-amber-700">
          <Pill tone="amber"><T>redacted</T></Pill>{" "}
          <T>Before/after values are hidden — you lack the sensitive-reader capability.</T>
        </p>
      ) : null}
      <ol className="space-y-2">
        {events.map((e, i) => {
          const key = `${e.at}-${e.requestId}-${i}`;
          const hasDiff = e.before != null || e.after != null;
          const isOpen = open.has(key);
          return (
            <li key={key} className="rounded-md border border-slate-200 px-3 py-2 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <span className="whitespace-nowrap text-xs text-slate-500">
                  {new Date(e.at).toLocaleString()}
                </span>
                <Mono>{e.action}</Mono>
                <span className="text-xs text-slate-400">{e.targetType}</span>
                <Pill tone={e.outcome === "SUCCESS" ? "green" : e.outcome === "DENIED" ? "amber" : "red"}>
                  {e.outcome}
                </Pill>
                <span className="ml-auto text-xs text-slate-500">
                  {e.actorType === "SYSTEM" ? (
                    <Pill><T>system</T></Pill>
                  ) : e.actorPersonId ? (
                    <Mono>{e.actorPersonId.slice(-8)}</Mono>
                  ) : (
                    "—"
                  )}
                </span>
                {hasDiff ? (
                  <button
                    type="button"
                    className="text-xs font-medium text-indigo-600 hover:underline"
                    onClick={() =>
                      setOpen((prev) => {
                        const next = new Set(prev);
                        next.has(key) ? next.delete(key) : next.add(key);
                        return next;
                      })
                    }
                  >
                    {isOpen ? <T>hide changes</T> : <T>show changes</T>}
                  </button>
                ) : null}
              </div>
              {isOpen && hasDiff ? (
                <div className="mt-2 grid gap-2 sm:grid-cols-2">
                  <Diff label={<T>Before</T>} value={e.before} />
                  <Diff label={<T>After</T>} value={e.after} />
                </div>
              ) : null}
            </li>
          );
        })}
      </ol>
      {token ? (
        <div className="mt-3 flex justify-center">
          <button type="button" className="btn-ghost" disabled={busy} onClick={() => fetchPage(token)}>
            {busy ? <T>Loading…</T> : <T>Load more</T>}
          </button>
        </div>
      ) : null}
    </div>
  );
}

function Diff({ label, value }: { label: React.ReactNode; value: unknown }) {
  return (
    <div>
      <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div>
      {value == null ? (
        <p className="text-xs text-slate-400">—</p>
      ) : (
        <pre className="overflow-x-auto rounded bg-slate-50 p-2 text-xs text-slate-700">
          {JSON.stringify(value, null, 2)}
        </pre>
      )}
    </div>
  );
}
