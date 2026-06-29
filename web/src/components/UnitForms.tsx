"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { ActionButton } from "./ActionButton";
import { T } from "./T";
import { useTg } from "@/lib/locale";
import type { Unit } from "@/lib/api/types";

/** Edit a unit's mutable fields + code (recode) + lifecycle transitions. PUT /tenant/v1/units/{id}. */
export function UnitAdmin({ unit }: { unit: Unit }) {
  const router = useRouter();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [codeOpen, setCodeOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const state = (unit.state ?? "").toUpperCase();

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button type="button" className="btn-ghost" onClick={() => setOpen((o) => !o)}>
          {open ? <T>Close</T> : <T>Edit unit</T>}
        </button>
        <button type="button" className="btn-ghost" onClick={() => setCodeOpen((o) => !o)}>
          {codeOpen ? <T>Close</T> : <T>Edit code</T>}
        </button>
        {state !== "SUSPENDED" && state !== "ARCHIVED" ? (
          <ActionButton
            method="POST"
            path={`/tenant/v1/units/${unit.id}/transition`}
            body={{ toState: "SUSPENDED" }}
            label="Suspend"
            confirm="Suspend this unit?"
          />
        ) : null}
        {state !== "ARCHIVED" ? (
          <ActionButton
            method="POST"
            path={`/tenant/v1/units/${unit.id}/transition`}
            body={{ toState: "ARCHIVED" }}
            label="Archive"
            confirm="Archive this unit? (the lifecycle equivalent of delete)"
            tone="danger"
          />
        ) : null}
        {state !== "ACTIVE" ? (
          <ActionButton
            method="POST"
            path={`/tenant/v1/units/${unit.id}/transition`}
            body={{ toState: "ACTIVE" }}
            label="Restore"
          />
        ) : null}
      </div>

      {open ? (
        <form
          className="card space-y-3 p-5"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            setBusy(true);
            setErr(null);
            (async () => {
              try {
                await api.tenant.updateUnit(unit.id, {
                  name: String(f.get("name") || "").trim() || undefined,
                  kindId: String(f.get("kindId") || "").trim() || undefined,
                  level: f.get("level") ? Number(f.get("level")) : undefined,
                  visibility: (String(f.get("visibility") || "").trim() || undefined) as never,
                });
                setOpen(false);
                router.refresh();
              } catch (e) {
                setErr(e);
              } finally {
                setBusy(false);
              }
            })();
          }}
        >
          {err ? <ErrorBox error={err} /> : null}
          <div>
            <label className="label"><T>Name</T></label>
            <input name="name" className="input" placeholder={tr("(unchanged)")} />
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="label"><T>Kind ID</T></label>
              <input name="kindId" className="input" defaultValue={unit.kindId ?? ""} placeholder={tr("unit-kind RID")} />
            </div>
            <div>
              <label className="label"><T>Level</T></label>
              <input name="level" type="number" className="input" defaultValue={unit.level ?? ""} />
            </div>
            <div>
              <label className="label"><T>Visibility</T></label>
              <select name="visibility" className="input" defaultValue={unit.visibility ?? "PUBLIC"}>
                <option value="PUBLIC">PUBLIC</option>
                <option value="SHADOW">SHADOW</option>
              </select>
            </div>
          </div>
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? <T>Saving…</T> : <T>Save</T>}
          </button>
        </form>
      ) : null}

      {codeOpen ? (
        <form
          className="card space-y-3 p-5"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            const code = String(f.get("code") || "").trim();
            const reason = String(f.get("reason") || "").trim();
            setBusy(true);
            setErr(null);
            (async () => {
              try {
                // An empty code clears it (the unit becomes a non-separate sub-unit; D-UnitCodeLifecycle).
                await api.tenant.setUnitCode(unit.id, { code: code || undefined, reason: reason || undefined });
                setCodeOpen(false);
                router.refresh();
              } catch (e) {
                setErr(e);
              } finally {
                setBusy(false);
              }
            })();
          }}
        >
          {err ? <ErrorBox error={err} /> : null}
          <p className="text-sm text-muted">
            <T>Set, correct, or clear the unit&apos;s code. Leave empty to clear it (a non-separate sub-unit). The RID stays the stable external handle.</T>
          </p>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label"><T>Code</T></label>
              <input name="code" className="input" defaultValue={unit.code ?? ""} placeholder={tr("(none)")} />
            </div>
            <div>
              <label className="label"><T>Reason</T></label>
              <input name="reason" className="input" placeholder={tr("(optional)")} />
            </div>
          </div>
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? <T>Saving…</T> : <T>Save code</T>}
          </button>
        </form>
      ) : null}
    </div>
  );
}
