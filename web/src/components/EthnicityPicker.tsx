"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";

/**
 * Hierarchical ethnicity picker (D-PhysicalIdentity amendment, M43), modeled on LanguagePicker. The
 * ethnicity taxonomy is a genealogical forest (group → sub-group), so this offers two ways in:
 *   • a lazily-expanded tree — roots via `topLevel`, a node's children via the `parent` filter, with an
 *     expand chevron shown only where `hasChildren`; and
 *   • a server `query` search returning flat matches as you type (debounced).
 * Any node is selectable. Because the encrypted person↔ethnicity write (`addEthnicity`) keys on the
 * catalog CODE, this picker yields the selected group's `code` (not its RID) via onChange / a hidden
 * <input name=…>. If the catalog is empty (default — the import is opt-in), the tree is simply empty.
 */
type EthnicityType = {
  id: string;
  code: string;
  name?: Record<string, string>;
  parentId?: string | null;
  hasChildren?: boolean;
};

export function EthnicityPicker({
  name,
  onChange,
  placeholder = "Search an ethnicity…",
}: {
  name?: string;
  onChange?: (code: string) => void;
  placeholder?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<EthnicityType[]>([]);
  const [searching, setSearching] = useState(false);
  const [roots, setRoots] = useState<EthnicityType[] | null>(null);
  const [childrenById, setChildrenById] = useState<Record<string, EthnicityType[]>>({});
  const [loadingId, setLoadingId] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState<{ code: string; label: string } | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  function loadRoots() {
    if (roots) return;
    api.person
      .listEthnicityTypes(true, undefined, undefined, 1000)
      .then((r) => setRoots((r ?? []) as unknown as EthnicityType[]))
      .catch(() => setRoots([]));
  }

  useEffect(() => {
    const q = query.trim();
    if (selected || q.length < 2) {
      setResults([]);
      return;
    }
    let alive = true;
    setSearching(true);
    const t = setTimeout(() => {
      api.person
        .listEthnicityTypes(undefined, undefined, q, 50)
        .then((r) => {
          if (!alive) return;
          setResults((r ?? []) as unknown as EthnicityType[]);
          setSearching(false);
        })
        .catch(() => alive && setSearching(false));
    }, 200);
    return () => {
      alive = false;
      clearTimeout(t);
    };
  }, [query, selected]);

  function toggle(node: EthnicityType) {
    const next = new Set(expanded);
    if (next.has(node.id)) {
      next.delete(node.id);
      setExpanded(next);
      return;
    }
    next.add(node.id);
    setExpanded(next);
    if (childrenById[node.id]) return;
    setLoadingId((s) => new Set(s).add(node.id));
    api.person
      .listEthnicityTypes(undefined, node.id, undefined, 1000)
      .then((r) => setChildrenById((m) => ({ ...m, [node.id]: (r ?? []) as unknown as EthnicityType[] })))
      .catch(() => setChildrenById((m) => ({ ...m, [node.id]: [] })))
      .finally(() => setLoadingId((s) => { const n = new Set(s); n.delete(node.id); return n; }));
  }

  function labelFor(e: EthnicityType) {
    return pickLabel(e.name, locale) || e.code || e.id;
  }
  function choose(e: EthnicityType) {
    setSelected({ code: e.code, label: labelFor(e) });
    setOpen(false);
    setQuery("");
    onChange?.(e.code);
  }
  function clear() {
    setSelected(null);
    setQuery("");
    onChange?.("");
  }

  function Row({ node, depth }: { node: EthnicityType; depth: number }) {
    const isExpanded = expanded.has(node.id);
    const isLoading = loadingId.has(node.id);
    const kids = childrenById[node.id] ?? [];
    return (
      <div>
        <div className="flex items-center gap-1 text-sm" style={{ paddingLeft: depth * 14 }}>
          {node.hasChildren ? (
            <button
              type="button"
              className="w-4 shrink-0 text-slate-400 hover:text-slate-700"
              onClick={() => toggle(node)}
              aria-label={isExpanded ? tr("collapse") : tr("expand")}
            >
              {isExpanded ? "▾" : "▸"}
            </button>
          ) : (
            <span className="w-4 shrink-0" />
          )}
          <button
            type="button"
            className="flex flex-1 items-center justify-between gap-2 truncate rounded px-1 py-0.5 text-left hover:bg-indigo-50"
            onClick={() => choose(node)}
          >
            <span className="truncate">{labelFor(node)}</span>
            <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">{node.code}</span>
          </button>
        </div>
        {isExpanded ? (
          isLoading ? (
            <div className="px-2 py-1 text-xs text-slate-400" style={{ paddingLeft: (depth + 1) * 14 }}>{tr("Loading…")}</div>
          ) : (
            kids.map((c) => <Row key={c.id} node={c} depth={depth + 1} />)
          )
        ) : null}
      </div>
    );
  }

  const showSearch = query.trim().length >= 2;

  return (
    <div className="relative" ref={boxRef}>
      {name ? <input type="hidden" name={name} value={selected?.code ?? ""} /> : null}
      {selected ? (
        <div className="flex items-center gap-2">
          <span className="input flex-1 truncate bg-slate-50">{selected.label}</span>
          <button type="button" className="text-xs text-red-600 hover:underline" onClick={clear}>{tr("clear")}</button>
        </div>
      ) : (
        <input
          className="input"
          placeholder={tr(placeholder)}
          value={query}
          autoComplete="off"
          onFocus={() => { loadRoots(); setOpen(true); }}
          onChange={(e) => { setQuery(e.target.value); setOpen(true); }}
        />
      )}
      {open && !selected ? (
        <div className="absolute z-10 mt-1 max-h-72 w-full overflow-auto rounded-md border border-slate-200 bg-white p-1 shadow-lg">
          {showSearch ? (
            searching ? (
              <div className="px-2 py-1.5 text-sm text-slate-400">{tr("Searching…")}</div>
            ) : results.length === 0 ? (
              <div className="px-2 py-1.5 text-sm text-slate-400">{tr("No matches")}</div>
            ) : (
              results.map((e) => (
                <button
                  key={e.id}
                  type="button"
                  className="flex w-full items-center justify-between truncate rounded px-2 py-1 text-left text-sm hover:bg-indigo-50"
                  onClick={() => choose(e)}
                >
                  <span className="truncate">{labelFor(e)}</span>
                  <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">{e.code}</span>
                </button>
              ))
            )
          ) : roots === null ? (
            <div className="px-2 py-1.5 text-sm text-slate-400">{tr("Loading…")}</div>
          ) : roots.length === 0 ? (
            <div className="px-2 py-1.5 text-sm text-slate-400">{tr("No ethnicities (import the catalog)")}</div>
          ) : (
            roots.map((r) => <Row key={r.id} node={r} depth={0} />)
          )}
        </div>
      ) : null}
    </div>
  );
}
