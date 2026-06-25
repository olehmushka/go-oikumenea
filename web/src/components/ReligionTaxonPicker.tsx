"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

/**
 * Expandable + searchable religion-taxonomy picker (D-Religion, M22). The faith taxonomy is a
 * recursive forest (religion → branch → tradition → sub-tradition → denomination), so unlike a flat
 * <select> this lets you drill in and select at any depth (e.g. Christianity › Protestantism ›
 * Lutheranism). The catalog is small (~hundreds of nodes, capped at 200/page server-side) so the
 * whole forest is loaded once on first open and the tree is built client-side — that gives exact
 * leaf detection (no expand chevron when a node has no children) and instant local search.
 *
 * Controlled: `onChange` submits the opaque taxon RID (or "" when cleared). Any taxon FKs into
 * `religion_affiliations.religion_id` regardless of rank.
 */
type Taxon = {
  id: string;
  code: string;
  name: LocaleMap;
  rankCode: string;
  parentId?: string;
  depth?: number;
};

export function ReligionTaxonPicker({
  value,
  onChange,
  placeholder = "faith (optional)…",
}: {
  value?: string;
  onChange: (id: string) => void;
  placeholder?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [nodes, setNodes] = useState<Taxon[] | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<{ id: string; label: string } | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  // Reset the chip if the controlled value is cleared by the parent (e.g. after a successful add).
  useEffect(() => {
    if (!value) setSelected(null);
  }, [value]);

  // Pull the whole forest once (paging through the 200/page cap), then build the tree client-side.
  async function loadForest() {
    if (nodes) return;
    try {
      const all: Taxon[] = [];
      let token: string | undefined;
      do {
        const page = await api.religion.listTaxa(undefined, undefined, undefined, undefined, 200, token);
        all.push(...((page?.taxa ?? []) as unknown as Taxon[]));
        token = page?.nextPageToken ?? undefined;
      } while (token);
      setNodes(all);
    } catch {
      setNodes([]);
    }
  }

  // children-by-parent map + the root list (nodes without a parent).
  const { childrenOf, roots } = useMemo(() => {
    const byParent = new Map<string, Taxon[]>();
    const rootList: Taxon[] = [];
    for (const n of nodes ?? []) {
      if (n.parentId) {
        const arr = byParent.get(n.parentId) ?? [];
        arr.push(n);
        byParent.set(n.parentId, arr);
      } else {
        rootList.push(n);
      }
    }
    return { childrenOf: (id: string) => byParent.get(id) ?? [], roots: rootList };
  }, [nodes]);

  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q || !nodes) return null;
    return nodes.filter((n) => (pickLabel(n.name, locale) || n.code).toLowerCase().includes(q) || n.code.toLowerCase().includes(q));
  }, [query, nodes, locale]);

  function toggle(id: string) {
    setExpanded((s) => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });
  }
  function choose(node: Taxon) {
    setSelected({ id: node.id, label: pickLabel(node.name, locale) || node.code });
    setOpen(false);
    setQuery("");
    onChange(node.id);
  }
  function clear() {
    setSelected(null);
    onChange("");
  }

  function Row({ node, depth }: { node: Taxon; depth: number }) {
    const kids = childrenOf(node.id);
    const isExpanded = expanded.has(node.id);
    return (
      <div>
        <div className="flex items-center gap-1 text-sm" style={{ paddingLeft: depth * 14 }}>
          {kids.length > 0 ? (
            <button
              type="button"
              className="w-4 shrink-0 text-slate-400 hover:text-slate-700"
              onClick={() => toggle(node.id)}
              aria-label={isExpanded ? tr("collapse") : tr("expand")}
            >
              {isExpanded ? "▾" : "▸"}
            </button>
          ) : (
            <span className="w-4 shrink-0" />
          )}
          <button
            type="button"
            className="flex-1 truncate rounded px-1 py-0.5 text-left hover:bg-indigo-50"
            onClick={() => choose(node)}
          >
            {pickLabel(node.name, locale) || node.code}
            <span className="ml-1.5 text-xs text-slate-400">{node.rankCode}</span>
          </button>
        </div>
        {isExpanded ? kids.map((c) => <Row key={c.id} node={c} depth={depth + 1} />) : null}
      </div>
    );
  }

  return (
    <div className="relative" ref={boxRef}>
      {selected ? (
        <div className="flex items-center gap-2">
          <span className="input flex-1 truncate bg-slate-50">{selected.label}</span>
          <button type="button" className="text-xs text-red-600 hover:underline" onClick={clear}>
            {tr("clear")}
          </button>
        </div>
      ) : (
        <button
          type="button"
          className="input w-full text-left text-slate-400"
          onClick={() => {
            loadForest();
            setOpen((o) => !o);
          }}
        >
          {tr(placeholder)}
        </button>
      )}
      {open && !selected ? (
        <div className="absolute z-10 mt-1 w-full rounded-md border border-slate-200 bg-white shadow-lg">
          <div className="border-b border-slate-100 p-1">
            <input
              className="input w-full"
              placeholder={tr("Search…")}
              value={query}
              autoFocus
              autoComplete="off"
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <div className="max-h-64 overflow-auto p-1">
            {nodes === null ? (
              <div className="px-2 py-1.5 text-sm text-slate-400">{tr("Loading…")}</div>
            ) : matches ? (
              matches.length === 0 ? (
                <div className="px-2 py-1.5 text-sm text-slate-400">{tr("No matches")}</div>
              ) : (
                // Flat results while searching: label + its rank, selectable directly.
                matches.map((n) => (
                  <button
                    key={n.id}
                    type="button"
                    className="flex w-full items-center justify-between truncate rounded px-2 py-1 text-left text-sm hover:bg-indigo-50"
                    onClick={() => choose(n)}
                  >
                    <span className="truncate">{pickLabel(n.name, locale) || n.code}</span>
                    <span className="ml-2 shrink-0 text-xs text-slate-400">{n.rankCode}</span>
                  </button>
                ))
              )
            ) : roots.length === 0 ? (
              <div className="px-2 py-1.5 text-sm text-slate-400">{tr("No religions")}</div>
            ) : (
              roots.map((r) => <Row key={r.id} node={r} depth={0} />)
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}
