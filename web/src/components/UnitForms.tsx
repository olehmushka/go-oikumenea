"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { ActionButton } from "./ActionButton";
import { T } from "./T";
import { useTg } from "@/lib/locale";
import type { Unit } from "@/lib/api/types";

/** Edit a unit's mutable fields (code is immutable) + lifecycle transitions. PUT /tenant/v1/units/{id}. */
export function UnitAdmin({ unit }: { unit: Unit }) {
  const router = useRouter();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const state = (unit.state ?? "").toUpperCase();

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button type="button" className="btn-ghost" onClick={() => setOpen((o) => !o)}>
          {open ? <T>Close</T> : <T>Edit unit</T>}
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
                  unitKind: String(f.get("unitKind") || "").trim() || undefined,
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
              <label className="label"><T>Kind</T></label>
              <input name="unitKind" className="input" defaultValue={unit.unitKind ?? ""} />
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
    </div>
  );
}
