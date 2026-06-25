"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";

/**
 * Glottolog language picker (D-Languages, M18). The languoid catalog is a ~27k-node genealogical
 * forest (family → language → dialect), so this offers two ways in:
 *   • a lazily-expanded tree — roots via `topLevel`, a node's children via the `parent` filter, with
 *     an expand chevron shown only where `hasChildren` (non-dialect children) is true; and
 *   • a server `query` search that returns flat language matches as you type (debounced).
 * Only `level='language'` nodes are selectable — families/sub-families are navigation-only — because
 * the SPEAKS / unit-language / locale links accept only languages. The opaque languoid RID is
 * submitted on select.
 *
 * Modes: `name` renders a hidden <input name=…> with the RID (FormData forms); `onChange` is the
 * controlled callback.
 */
type Languoid = {
  id: string;
  code?: string;
  level?: string;
  name?: Record<string, string>;
  parentId?: string | null;
  hasChildren?: boolean;
  iso6393?: string | null;
};

export function LanguagePicker({
  name,
  onChange,
  placeholder = "Search a language…",
}: {
  name?: string;
  onChange?: (id: string) => void;
  placeholder?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Languoid[]>([]);
  const [searching, setSearching] = useState(false);
  const [roots, setRoots] = useState<Languoid[] | null>(null);
  const [childrenById, setChildrenById] = useState<Record<string, Languoid[]>>({});
  const [loadingId, setLoadingId] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState<{ id: string; label: string } | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  // Lazy-load the forest roots the first time the popover opens.
  function loadRoots() {
    if (roots) return;
    api.language
      .listLanguages(undefined, undefined, undefined, true, undefined, 1000)
      .then((r) => setRoots((r?.languoids ?? []) as unknown as Languoid[]))
      .catch(() => setRoots([]));
  }

  // Debounced server search (level=language) while the query has ≥2 chars.
  useEffect(() => {
    const q = query.trim();
    if (selected || q.length < 2) {
      setResults([]);
      return;
    }
    let alive = true;
    setSearching(true);
    const t = setTimeout(() => {
      api.language
        .listLanguages("language", undefined, undefined, undefined, q, 50)
        .then((r) => {
          if (!alive) return;
          setResults((r?.languoids ?? []) as unknown as Languoid[]);
          setSearching(false);
        })
        .catch(() => alive && setSearching(false));
    }, 200);
    return () => {
      alive = false;
      clearTimeout(t);
    };
  }, [query, selected]);

  function toggle(node: Languoid) {
    const next = new Set(expanded);
    if (next.has(node.id)) {
      next.delete(node.id);
      setExpanded(next);
      return;
    }
    next.add(node.id);
    setExpanded(next);
    if (childrenById[node.id]) return; // cached
    setLoadingId((s) => new Set(s).add(node.id));
    api.language
      .listLanguages(undefined, undefined, node.id, undefined, undefined, 1000)
      .then((r) => setChildrenById((m) => ({ ...m, [node.id]: (r?.languoids ?? []) as unknown as Languoid[] })))
      .catch(() => setChildrenById((m) => ({ ...m, [node.id]: [] })))
      .finally(() => setLoadingId((s) => { const n = new Set(s); n.delete(node.id); return n; }));
  }

  function labelFor(l: Languoid) {
    return pickLabel(l.name, locale) || l.code || l.id;
  }
  function choose(l: Languoid) {
    setSelected({ id: l.id, label: `${labelFor(l)}${l.iso6393 ? ` (${l.iso6393})` : ""}` });
    setOpen(false);
    setQuery("");
    onChange?.(l.id);
  }
  function clear() {
    setSelected(null);
    setQuery("");
    onChange?.("");
  }

  function Row({ node, depth }: { node: Languoid; depth: number }) {
    const isLanguage = node.level === "language";
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
            className={
              "flex flex-1 items-center justify-between gap-2 truncate rounded px-1 py-0.5 text-left " +
              (isLanguage ? "hover:bg-indigo-50" : "text-slate-500 hover:bg-slate-50")
            }
            onClick={() => (isLanguage ? choose(node) : node.hasChildren ? toggle(node) : undefined)}
          >
            <span className="truncate">{labelFor(node)}</span>
            <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">
              {isLanguage ? node.iso6393 || node.code : node.level}
            </span>
          </button>
        </div>
        {isExpanded ? (
          isLoading ? (
            <div className="px-2 py-1 text-xs text-slate-400" style={{ paddingLeft: (depth + 1) * 14 }}>
              {tr("Loading…")}
            </div>
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
      {name ? <input type="hidden" name={name} value={selected?.id ?? ""} /> : null}
      {selected ? (
        <div className="flex items-center gap-2">
          <span className="input flex-1 truncate bg-slate-50">{selected.label}</span>
          <button type="button" className="text-xs text-red-600 hover:underline" onClick={clear}>
            {tr("clear")}
          </button>
        </div>
      ) : (
        <input
          className="input"
          placeholder={tr(placeholder)}
          value={query}
          autoComplete="off"
          onFocus={() => {
            loadRoots();
            setOpen(true);
          }}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
          }}
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
              results.map((l) => (
                <button
                  key={l.id}
                  type="button"
                  className="flex w-full items-center justify-between truncate rounded px-2 py-1 text-left text-sm hover:bg-indigo-50"
                  onClick={() => choose(l)}
                >
                  <span className="truncate">{labelFor(l)}</span>
                  <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">{l.iso6393 || l.code}</span>
                </button>
              ))
            )
          ) : roots === null ? (
            <div className="px-2 py-1.5 text-sm text-slate-400">{tr("Loading…")}</div>
          ) : roots.length === 0 ? (
            <div className="px-2 py-1.5 text-sm text-slate-400">{tr("No languages")}</div>
          ) : (
            roots.map((r) => <Row key={r.id} node={r} depth={0} />)
          )}
        </div>
      ) : null}
    </div>
  );
}
