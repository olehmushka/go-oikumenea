"use client";

import { useState } from "react";
import { mutate } from "@/lib/api/client";
import { PageHeader } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { EntitySelect } from "@/components/EntitySelect";
import type { AuthorizeResponse } from "@/lib/api/types";

export default function AuthorizePage() {
  const [result, setResult] = useState<AuthorizeResponse | null>(null);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    setResult(null);
    const f = new FormData(e.currentTarget);
    const body = {
      subjectPersonId: String(f.get("subjectPersonId") || "").trim(),
      action: String(f.get("action") || "").trim(),
      unitId: String(f.get("unitId") || "").trim() || undefined,
      explain: true,
    };
    try {
      const r = await mutate<AuthorizeResponse>(
        "POST",
        "/authorization/v1/authorize",
        body,
      );
      setResult(r);
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="max-w-2xl">
      <PageHeader
        title="Authorize"
        description="Ask the PDP a single question: may this person perform this action on this unit? This is the same decision the service makes for every API request."
      />

      <form onSubmit={onSubmit} className="card space-y-4 p-5">
        <div>
          <label className="label">Subject person *</label>
          <EntitySelect name="subjectPersonId" kind="person" required placeholder="Search a person…" />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label">Action (permission code) *</label>
            <input name="action" required className="input font-mono" placeholder="person.read" />
          </div>
          <div>
            <label className="label">Unit</label>
            <EntitySelect name="unitId" kind="unit" allowEmpty placeholder="(optional) search a unit…" />
          </div>
        </div>
        <button type="submit" className="btn-primary" disabled={busy}>
          {busy ? "Deciding…" : "Run decision"}
        </button>
      </form>

      {err ? <div className="mt-4"><ErrorBox error={err} /></div> : null}

      {result && (
        <div className="mt-4">
          <div
            className={`card p-5 ${
              result.allow ? "border-green-200 bg-green-50" : "border-red-200 bg-red-50"
            }`}
          >
            <div
              className={`text-lg font-semibold ${
                result.allow ? "text-green-800" : "text-red-800"
              }`}
            >
              {result.allow ? "ALLOW" : "DENY"}
            </div>
            {result.explanation?.reason && (
              <div className="mt-1 text-sm text-slate-600">
                {result.explanation.reason}
              </div>
            )}
            {result.explanation?.instanceAdmin && (
              <div className="mt-1 text-sm text-indigo-700">
                Granted via the instance-admin plane.
              </div>
            )}
            {result.explanation?.contributions &&
              result.explanation.contributions.length > 0 && (
                <div className="mt-3">
                  <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Contributing assignments
                  </div>
                  <ul className="mt-1 space-y-1 text-sm text-slate-700">
                    {result.explanation.contributions.map((c, i) => (
                      <li key={i} className="font-mono text-xs">
                        role={c.roleCode ?? c.assignmentId} · scope={c.scope}
                        {c.viaUnitId ? ` · via ${c.viaUnitId.slice(-8)}` : ""}
                        {c.graphId ? ` · graph ${c.graphId.slice(-8)}` : ""}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
          </div>
        </div>
      )}
    </div>
  );
}
