"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "./ErrorBox";
import { EntitySelect } from "./EntitySelect";
import { ActionButton } from "./ActionButton";
import { pickLabel } from "@/lib/i18n";
import { useLocale } from "@/lib/locale";
import type { Position } from "@/lib/api/types";

/** Create a (vacant) billet in a unit. POST /membership/v1/units/{unitId}/positions. */
export function CreatePosition({ unitId }: { unitId: string }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        setBusy(true);
        setErr(null);
        (async () => {
          try {
            await mutate("POST", `/membership/v1/units/${unitId}/positions`, {
              code: String(f.get("code") || "").trim(),
              title: String(f.get("title") || "").trim(),
            });
            form.reset();
            router.refresh();
          } catch (e) {
            setErr(e);
          } finally {
            setBusy(false);
          }
        })();
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">Create position</h3>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input name="code" required className="input" placeholder="code (e.g. cmd-officer)" />
        <input name="title" required className="input" placeholder="title (e.g. Commanding Officer)" />
      </div>
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? "Creating…" : "Create position"}
      </button>
    </form>
  );
}

/** Edit a position's title / sort order, and abolish/restore it. */
export function PositionAdmin({ position }: { position: Position }) {
  const { locale } = useLocale();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const abolished = (position.status ?? "").toLowerCase() === "abolished";

  return (
    <span className="inline-flex items-center gap-3">
      <button
        type="button"
        className="text-xs font-medium text-indigo-600 hover:underline"
        onClick={() => setOpen((o) => !o)}
      >
        Edit
      </button>
      {!abolished ? (
        <ActionButton
          method="POST"
          path={`/membership/v1/positions/${position.id}/abolish`}
          label="Abolish"
          confirm="Abolish this billet? (end any holder first)"
          tone="danger"
        />
      ) : null}
      {open ? (
        <form
          className="absolute z-20 mt-1 w-72 space-y-2 rounded-md border border-slate-200 bg-white p-3 shadow-lg"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            setBusy(true);
            setErr(null);
            (async () => {
              try {
                await mutate("PUT", `/membership/v1/positions/${position.id}`, {
                  title: String(f.get("title") || "").trim() || undefined,
                  sortOrder: f.get("sortOrder") ? Number(f.get("sortOrder")) : undefined,
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
          <input
            name="title"
            className="input"
            placeholder="title"
            defaultValue={pickLabel(position.title, locale)}
          />
          <input
            name="sortOrder"
            type="number"
            className="input"
            placeholder="sort order"
            defaultValue={position.sortOrder ?? undefined}
          />
          <div className="flex gap-2">
            <button className="btn-primary" disabled={busy}>
              Save
            </button>
            <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
              Cancel
            </button>
          </div>
        </form>
      ) : null}
    </span>
  );
}

/** Assign a person to a vacant position. POST /membership/v1/positions/{positionId}/fill. */
export function FillPosition({ positionId }: { positionId: string }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [personId, setPersonId] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  if (!open) {
    return (
      <button
        type="button"
        className="text-xs font-medium text-indigo-600 hover:underline"
        onClick={() => setOpen(true)}
      >
        Fill
      </button>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <div className="w-56">
        <EntitySelect kind="person" placeholder="person…" onChange={setPersonId} allowEmpty />
      </div>
      <button
        type="button"
        className="btn-primary"
        disabled={busy || !personId}
        onClick={async () => {
          setBusy(true);
          setErr(null);
          try {
            await mutate("POST", `/membership/v1/positions/${positionId}/fill`, { personId });
            setOpen(false);
            router.refresh();
          } catch (e) {
            setErr((e as { errorName?: string })?.errorName ?? "Failed");
            setBusy(false);
          }
        }}
      >
        {busy ? "…" : "Assign"}
      </button>
      <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
        Cancel
      </button>
      {err && <span className="text-xs text-red-500">{err}</span>}
    </div>
  );
}
