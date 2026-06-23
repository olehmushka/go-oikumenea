"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { EntitySelect } from "./EntitySelect";
import { GraphSelect } from "./GraphSelect";
import { Localized } from "./Localized";
import { ErrorBox } from "./ErrorBox";
import { T } from "./T";
import { useTg } from "@/lib/locale";
import type { UnitRef } from "@/lib/api/types";

/**
 * Add / remove parent AND child edges for a unit within a graph — the UI for building the unit DAG.
 *  - Add parent: attaches THIS unit as a child of the picked unit (POST /units/{this}/edges).
 *  - Add child:  attaches the picked unit as a child of THIS unit  (POST /units/{picked}/edges).
 * Parents are the graph's ancestors, children its descendants. Remove targets a direct edge.
 * Graph "" means the default `command` graph.
 */
export function EdgeManager({ unitId }: { unitId: string }) {
  const router = useRouter();
  const [graph, setGraph] = useState("");
  const [parents, setParents] = useState<UnitRef[]>([]);
  const [children, setChildren] = useState<UnitRef[]>([]);
  const [parentId, setParentId] = useState("");
  const [childId, setChildId] = useState("");
  const [selKey, setSelKey] = useState(0); // bump to reset the pickers after a successful add
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);

  const load = useCallback(async () => {
    const gq = graph ? `?graph=${encodeURIComponent(graph)}` : "";
    const get = async (rel: "ancestors" | "descendants") => {
      try {
        const d =
          rel === "ancestors"
            ? await api.tenant.unitAncestors(unitId, graph || undefined)
            : await api.tenant.unitDescendants(unitId, graph || undefined);
        return d.units ?? [];
      } catch {
        return [];
      }
    };
    setParents(await get("ancestors"));
    setChildren(await get("descendants"));
  }, [unitId, graph]);

  useEffect(() => {
    load();
  }, [load]);

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      setParentId("");
      setChildId("");
      setSelKey((k) => k + 1);
      await load();
      router.refresh();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  const addParent = () =>
    parentId &&
    run(() => api.tenant.addEdge(unitId, { parentId, graph: graph || undefined }));
  const addChild = () =>
    childId &&
    run(() => api.tenant.addEdge(childId, { parentId: unitId, graph: graph || undefined }));

  const removeEdge = (childUnit: string, parentUnit: string) => {
    if (!window.confirm("Remove this edge?")) return;
    run(() => api.tenant.removeEdge(childUnit, parentUnit, graph || undefined));
  };

  return (
    <div className="space-y-4">
      <div className="max-w-xs">
        <label className="label"><T>Graph</T></label>
        <GraphSelect value={graph} onChange={setGraph} />
      </div>

      <EdgeSide
        title="Parents"
        addLabel="Add parent"
        placeholder="Search a parent unit…"
        selKey={`p${selKey}`}
        busy={busy}
        value={parentId}
        onPick={setParentId}
        onAdd={addParent}
        units={parents}
        emptyText="No parents in this graph (a root unit)."
        onRemove={(u) => removeEdge(unitId, u)} // remove THIS unit's edge to that parent
      />

      <EdgeSide
        title="Children"
        addLabel="Add child"
        placeholder="Search a child unit…"
        selKey={`c${selKey}`}
        busy={busy}
        value={childId}
        onPick={setChildId}
        onAdd={addChild}
        units={children}
        emptyText="No children in this graph."
        onRemove={(u) => removeEdge(u, unitId)} // remove that child's edge to THIS unit
      />

      {err ? <ErrorBox error={err} /> : null}
      <p className="text-xs text-slate-400">
        <T>Lists show the graph's ancestors / descendants; remove targets a direct edge (a transitive relation must be detached at its own edge).</T>
      </p>
    </div>
  );
}

function EdgeSide({
  title,
  addLabel,
  placeholder,
  selKey,
  busy,
  value,
  onPick,
  onAdd,
  units,
  emptyText,
  onRemove,
}: {
  title: string;
  addLabel: string;
  placeholder: string;
  selKey: string;
  busy: boolean;
  value: string;
  onPick: (id: string) => void;
  onAdd: () => void;
  units: UnitRef[];
  emptyText: string;
  onRemove: (unitId: string) => void;
}) {
  const tr = useTg();
  return (
    <div className="rounded-md border border-slate-200 p-3">
      <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">
        {tr(title)}
      </div>
      <div className="grid items-end gap-2 sm:grid-cols-[1fr_auto]">
        <EntitySelect key={selKey} kind="unit" placeholder={tr(placeholder)} allowEmpty onChange={onPick} />
        <button type="button" className="btn-primary" disabled={busy || !value} onClick={onAdd}>
          {busy ? "…" : tr(addLabel)}
        </button>
      </div>
      {units.length > 0 ? (
        <ul className="mt-3 space-y-1 text-sm">
          {units.map((u) => (
            <li key={u.id} className="flex items-center justify-between gap-2">
              <Localized map={u.name} fallback={u.code ?? undefined} />
              <button
                type="button"
                className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
                disabled={busy}
                onClick={() => onRemove(u.id)}
              >
                <T>Remove</T>
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-3 text-sm text-slate-400">{tr(emptyText)}</p>
      )}
    </div>
  );
}
