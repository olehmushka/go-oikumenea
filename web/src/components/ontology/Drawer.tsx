"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { mutate } from "@/lib/api/client";
import { bffGet, resolveLinkGroups } from "@/lib/api/browser";
import { OBJECT_TYPES, type ActionDef, type Row } from "@/lib/ontology/registry";
import { pushRecent } from "@/lib/ontology/recents";
import { ObjectHeader } from "./ObjectHeader";
import { PropertyList } from "./PropertyList";
import { LinksPanel, type LinkGroup } from "./LinksPanel";

/** A right-side detail panel for one object — properties, links, and inline actions, without leaving
 *  the list. `type`/`id` are strings (serializable); the registry is looked up here in the browser. */
export function Drawer({
  type,
  id,
  onClose,
  onActed,
}: {
  type: string;
  id: string;
  onClose: () => void;
  onActed?: () => void;
}) {
  const def = OBJECT_TYPES[type];
  const [obj, setObj] = useState<Row | null>(null);
  const [groups, setGroups] = useState<LinkGroup[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    setObj(null);
    setErr(null);
    setGroups([]);
    if (!def?.get) {
      setErr("This type has no detail endpoint.");
      return;
    }
    bffGet<Row>(def.get(id))
      .then((o) => {
        if (!alive) return;
        setObj(o);
        pushRecent({ id: o.id ?? id, type, label: def.title(o) });
      })
      .catch(() => alive && setErr("Could not load object."));
    resolveLinkGroups(def, id).then((g) => alive && setGroups(g));
    return () => {
      alive = false;
    };
  }, [def, id, type]);

  // Close on Escape.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  const runAction = async (a: ActionDef) => {
    if (a.confirm && !window.confirm(a.confirm)) return;
    setBusy(true);
    setErr(null);
    try {
      await mutate(a.method, a.path(id), a.body?.());
      onActed?.();
      // refresh the object after acting
      if (def?.get) setObj(await bffGet<Row>(def.get(id)));
    } catch {
      setErr("Action failed.");
    } finally {
      setBusy(false);
    }
  };

  const actions = (obj && def?.actions ? def.actions.filter((a) => !a.appliesTo || a.appliesTo(obj)) : []);

  return (
    <>
      <div className="fixed inset-0 z-30 bg-slate-900/20" onClick={onClose} />
      <aside className="fixed right-0 top-0 z-40 flex h-full w-full max-w-md flex-col border-l border-slate-200 bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <span className="text-xs font-semibold uppercase tracking-wide text-slate-400">Detail</span>
          <div className="flex items-center gap-3">
            <Link href={`/o/${encodeURIComponent(id)}`} className="text-xs text-indigo-600 hover:underline">
              Full view →
            </Link>
            <button onClick={onClose} className="text-slate-400 hover:text-slate-700" aria-label="Close">
              ✕
            </button>
          </div>
        </div>

        <div className="flex-1 space-y-5 overflow-y-auto p-4">
          {err ? <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">{err}</div> : null}
          {!obj && !err ? <div className="text-sm text-slate-400">Loading…</div> : null}

          {obj && def ? (
            <>
              <ObjectHeader def={def} obj={obj} />

              {actions.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {actions.map((a) => (
                    <button
                      key={a.key}
                      disabled={busy}
                      onClick={() => runAction(a)}
                      className={a.danger ? "btn-ghost border-red-300 text-red-700" : "btn-ghost"}
                    >
                      {a.label}
                    </button>
                  ))}
                </div>
              ) : null}

              <div className="card p-4">
                <h2 className="mb-3 text-sm font-semibold text-slate-900">Properties</h2>
                <PropertyList def={def} obj={obj} />
              </div>

              <div className="card p-4">
                <h2 className="mb-3 text-sm font-semibold text-slate-900">Links</h2>
                <LinksPanel groups={groups} />
              </div>
            </>
          ) : null}
        </div>
      </aside>
    </>
  );
}
