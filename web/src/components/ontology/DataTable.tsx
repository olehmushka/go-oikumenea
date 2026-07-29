"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { useLocale } from "@/lib/locale";
import { setActiveLocale } from "@/lib/i18n";
import { tg } from "@/lib/messages";
import { OBJECT_TYPES, rowSearchText, type Row } from "@/lib/ontology/registry";
import { Value } from "./Value";
import { Drawer } from "./Drawer";

/** Registry-driven object table: client sort, multi-select + bulk actions, and a detail drawer on row
 *  click (keeps your place in the list). `type` is a string; the def is looked up here.
 *
 *  Filtering moved to the URL in M56 ticket 4 (FilterBar): a type whose list endpoint ships a search
 *  arg is narrowed SERVER-side, so no box is drawn here. Types without one keep a page-local box,
 *  labelled as such — it only ever sees the rows already fetched, and the old "N of M" counter beside
 *  a keyset-paginated table read as a total, which it never was.
 *
 *  Column sort is still page-local for the same reason and cannot be honestly moved to the URL: no
 *  list endpoint ships an `orderBy` arg (see the Sort open seam in docs/architecture/facets.md). */
export function DataTable({
  type,
  rows,
}: {
  type: string;
  rows: Row[];
}) {
  const def = OBJECT_TYPES[type];
  const router = useRouter();
  // Subscribe to the UI locale so the table re-renders (and re-reads name maps) when it switches.
  const { locale } = useLocale();
  setActiveLocale(locale);
  const [filter, setFilter] = useState("");
  const [sortKey, setSortKey] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<1 | -1>(1);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [sel, setSel] = useState<string | null>(null); // drawer target rid
  const [busy, setBusy] = useState(false);

  const cols = def?.columns ?? [];
  // Only for types the backend cannot narrow; otherwise the FilterBar's search box is the real one.
  const pageLocalFilter = !def?.list?.searchParam;
  // A type with no detail endpoint (link__member_of — the contract ships no GET /memberships/{id})
  // must not open a drawer that immediately errors.
  const openable = Boolean(def?.get);

  // Stable per-row identity. Structural objects carry an `id` (RID); catalog/locale objects are
  // keyed by their locale-agnostic `code` instead. Fall back to a positional key so React keys
  // stay unique even if a row has neither.
  const ridOf = (r: Row): string => r.id || (typeof r.code === "string" ? r.code : "");

  const view = useMemo(() => {
    const q = filter.trim().toLowerCase();
    let out = q ? rows.filter((r) => def && rowSearchText(def, r).includes(q)) : rows.slice();
    if (sortKey) {
      const col = cols.find((c) => c.key === sortKey);
      if (col) {
        out = out.sort((a, b) => {
          const av = col.value(a) ?? "";
          const bv = col.value(b) ?? "";
          return String(av).localeCompare(String(bv), undefined, { numeric: true }) * sortDir;
        });
      }
    }
    return out;
  }, [rows, filter, sortKey, sortDir, cols, def, locale]);

  if (!def) return <p className="text-sm text-red-600">Unknown type: {type}</p>;

  const toggleSort = (key: string) => {
    if (sortKey === key) setSortDir((d) => (d === 1 ? -1 : 1));
    else {
      setSortKey(key);
      setSortDir(1);
    }
  };

  const allShown = view.length > 0 && view.every((r) => selected.has(ridOf(r)));
  const toggleAll = () =>
    setSelected(allShown ? new Set() : new Set(view.map(ridOf)));
  const toggleOne = (id: string) =>
    setSelected((s) => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });

  const runBulk = async (actionKey: string) => {
    const a = def.actions?.find((x) => x.key === actionKey);
    if (!a) return;
    const ids = [...selected];
    if (!window.confirm(`${a.label} ${ids.length} ${def.label.toLowerCase()}(s)?`)) return;
    setBusy(true);
    try {
      await Promise.allSettled(ids.map((id) => api.request(a.method, a.path(id), { body: a.body?.() })));
      setSelected(new Set());
      router.refresh();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      {pageLocalFilter ? (
        <div className="mb-3 flex items-center gap-3">
          <input
            className="input max-w-xs"
            placeholder={tg("Filter rows on this page…")}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          <span className="text-xs text-slate-400">
            {view.length} {tg("of")} {rows.length} {tg("on this page")}
          </span>
        </div>
      ) : null}

      {selected.size > 0 && (def.actions?.length ?? 0) > 0 ? (
        <div className="mb-3 flex items-center gap-2 rounded-md border border-indigo-200 bg-indigo-50 px-3 py-2 text-sm">
          <span className="font-medium text-indigo-800">{selected.size} {tg("selected")}</span>
          <div className="ml-2 flex gap-2">
            {def.actions!.map((a) => (
              <button
                key={a.key}
                disabled={busy}
                onClick={() => runBulk(a.key)}
                className={a.danger ? "btn-ghost border-red-300 text-red-700" : "btn-ghost"}
              >
                {tg(a.label)}
              </button>
            ))}
          </div>
          <button className="ml-auto text-xs text-slate-500 hover:underline" onClick={() => setSelected(new Set())}>
            {tg("Clear")}
          </button>
        </div>
      ) : null}

      <div className="card overflow-hidden">
        <table className="w-full">
          <thead className="border-b border-slate-200 bg-slate-50">
            <tr>
              {(def.actions?.length ?? 0) > 0 ? (
                <th className="th w-8">
                  <input type="checkbox" checked={allShown} onChange={toggleAll} aria-label="Select all" />
                </th>
              ) : null}
              {cols.map((c) => (
                <th
                  key={c.key}
                  className="th cursor-pointer select-none hover:text-slate-700"
                  onClick={() => toggleSort(c.key)}
                >
                  {tg(c.header)}
                  {sortKey === c.key ? <span className="ml-1">{sortDir === 1 ? "▲" : "▼"}</span> : null}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {view.map((r, i) => {
              const rid = ridOf(r);
              return (
              <tr
                key={rid || `row-${i}`}
                className={`hover:bg-slate-50 ${openable ? "cursor-pointer" : ""}`}
                onClick={openable ? () => setSel(rid) : undefined}
              >
                {(def.actions?.length ?? 0) > 0 ? (
                  <td className="td" onClick={(e) => e.stopPropagation()}>
                    <input type="checkbox" checked={selected.has(rid)} onChange={() => toggleOne(rid)} />
                  </td>
                ) : null}
                {cols.map((c) => (
                  <td key={c.key} className={`td ${c.align === "right" ? "text-right" : ""}`}>
                    <Value value={c.value(r)} render={c.render} tone={c.tone?.(r)} />
                  </td>
                ))}
              </tr>
              );
            })}
            {view.length === 0 ? (
              <tr>
                <td className="td text-slate-400" colSpan={cols.length + ((def.actions?.length ?? 0) > 0 ? 1 : 0)}>
                  {tg("No matching rows.")}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {sel ? (
        <Drawer type={type} id={sel} onClose={() => setSel(null)} onActed={() => router.refresh()} />
      ) : null}
    </div>
  );
}
