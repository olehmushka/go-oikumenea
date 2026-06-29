"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { useLocale } from "@/lib/locale";
import { setActiveLocale } from "@/lib/i18n";
import { tg } from "@/lib/messages";
import { OBJECT_TYPES, type Row } from "@/lib/ontology/registry";
import { Value } from "./Value";
import { Drawer } from "./Drawer";

// One level's worth of units; the hierarchy primitives are small per node, so a generous page keeps
// expansion to a single request in practice.
const PAGE = 200;

const unitDef = OBJECT_TYPES["unit"];

// The compact pill columns shown on each tree row (reuse the registry's render hints/tones).
const PILL_COLS = (unitDef.columns ?? []).filter((c) => c.key === "visibility" || c.key === "state");

type GraphOpt = { code: string; isDefault: boolean };

/**
 * Hierarchical (expand-on-click) view of an organization's units, backed by the tenant hierarchy
 * endpoints (D-TenantOrganizations). Roots come from GET /units?rootsOnly=true; each node's direct
 * children from GET /units?parent=<rid>. Both are scoped to the selected graph (default command).
 * A unit with multiple parents (the DAG) legitimately appears under each — nodes are keyed by path.
 */
export function UnitTree({ orgId }: { orgId: string }) {
  const { locale } = useLocale();
  setActiveLocale(locale);
  const router = useRouter();

  const [graphs, setGraphs] = useState<GraphOpt[]>([]);
  const [graph, setGraph] = useState("command");
  const [roots, setRoots] = useState<Row[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sel, setSel] = useState<string | null>(null); // drawer target rid

  // Load the org's graph registry once; default to the org's default graph (seeded "command").
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await api.request<{ graphs?: Array<Record<string, unknown>> }>(
          "GET",
          `/tenant/v1/organizations/${encodeURIComponent(orgId)}/graphs`,
        );
        if (cancelled) return;
        const gs: GraphOpt[] = (res.graphs ?? []).map((g) => ({ code: String(g.code), isDefault: !!g.isDefault }));
        setGraphs(gs);
        setGraph(gs.find((g) => g.isDefault)?.code ?? gs[0]?.code ?? "command");
      } catch {
        if (!cancelled) setGraph("command");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [orgId]);

  // (Re)load roots whenever the org or graph changes.
  useEffect(() => {
    let cancelled = false;
    setRoots(null);
    setError(null);
    (async () => {
      try {
        const res = await api.request("GET", "/tenant/v1/units", {
          query: { org: orgId, rootsOnly: true, graph, pageSize: PAGE },
        });
        if (!cancelled) setRoots(unitDef.list!.parse(res).rows);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [orgId, graph]);

  return (
    <div>
      {graphs.length > 1 ? (
        <div className="mb-3 flex items-center gap-2">
          <label className="label mb-0">{tg("Graph")}</label>
          <select className="input max-w-xs" value={graph} onChange={(e) => setGraph(e.target.value)}>
            {graphs.map((g) => (
              <option key={g.code} value={g.code}>
                {g.code}
              </option>
            ))}
          </select>
        </div>
      ) : null}

      {error ? (
        <p className="text-sm text-red-600">{error}</p>
      ) : roots === null ? (
        <p className="text-sm text-slate-400">{tg("Loading…")}</p>
      ) : roots.length === 0 ? (
        <p className="text-sm text-slate-400">{tg("No top-level units in this graph.")}</p>
      ) : (
        <div className="card overflow-hidden py-1">
          {roots.map((u) => (
            <TreeNode key={u.id} unit={u} orgId={orgId} graph={graph} depth={0} path={u.id} onSelect={setSel} />
          ))}
        </div>
      )}

      {sel ? (
        <Drawer type="unit" id={sel} onClose={() => setSel(null)} onActed={() => router.refresh()} />
      ) : null}
    </div>
  );
}

function TreeNode({
  unit,
  orgId,
  graph,
  depth,
  path,
  onSelect,
}: {
  unit: Row;
  orgId: string;
  graph: string;
  depth: number;
  path: string;
  onSelect: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [children, setChildren] = useState<Row[] | null>(null);
  const [loading, setLoading] = useState(false);

  const toggle = async () => {
    const next = !open;
    setOpen(next);
    if (next && children === null) {
      setLoading(true);
      try {
        const res = await api.request("GET", "/tenant/v1/units", {
          query: { org: orgId, parent: unit.id, graph, pageSize: PAGE },
        });
        setChildren(unitDef.list!.parse(res).rows);
      } catch {
        setChildren([]);
      } finally {
        setLoading(false);
      }
    }
  };

  const title = unitDef.title(unit);
  const subtitle = unitDef.subtitle?.(unit);

  return (
    <div>
      <div
        className="flex items-center gap-2 border-b border-slate-100 py-1.5 pr-3 last:border-0 hover:bg-slate-50"
        style={{ paddingLeft: depth * 20 + 8 }}
      >
        <button
          type="button"
          onClick={toggle}
          aria-label={open ? "Collapse" : "Expand"}
          className="w-4 shrink-0 text-slate-400 hover:text-slate-700"
        >
          {open ? "▾" : "▸"}
        </button>
        <button type="button" className="flex flex-1 items-center gap-3 text-left" onClick={() => onSelect(unit.id)}>
          <span className="font-mono text-xs text-slate-500">{title}</span>
          {subtitle ? <span className="text-sm text-slate-800">{subtitle}</span> : null}
          <span className="ml-auto flex items-center gap-2">
            {PILL_COLS.map((c) => (
              <Value key={c.key} value={c.value(unit)} render={c.render} tone={c.tone?.(unit)} />
            ))}
          </span>
        </button>
      </div>
      {open ? (
        loading ? (
          <div className="py-1.5 text-xs text-slate-400" style={{ paddingLeft: (depth + 1) * 20 + 24 }}>
            {tg("Loading…")}
          </div>
        ) : children && children.length > 0 ? (
          children.map((c) => (
            <TreeNode key={`${path}/${c.id}`} unit={c} orgId={orgId} graph={graph} depth={depth + 1} path={`${path}/${c.id}`} onSelect={onSelect} />
          ))
        ) : (
          <div className="py-1.5 text-xs italic text-slate-300" style={{ paddingLeft: (depth + 1) * 20 + 24 }}>
            {tg("No sub-units.")}
          </div>
        )
      ) : null}
    </div>
  );
}
