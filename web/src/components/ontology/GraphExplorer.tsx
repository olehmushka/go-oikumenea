"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Background,
  Controls,
  ReactFlow,
  type Edge,
  type Node,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { bffGet, resolveLinkGroups } from "@/lib/api/browser";
import { OBJECT_TYPES, type Row } from "@/lib/ontology/registry";
import { parseRid } from "@/lib/ontology/rid";

interface NodeData extends Record<string, unknown> {
  type: string;
  label: string;
}

const TYPE_COLOR: Record<string, string> = {
  person: "#6366f1",
  unit: "#0ea5e9",
  order: "#f59e0b",
  position: "#10b981",
  document: "#8b5cf6",
};

function nodeStyle(type: string): React.CSSProperties {
  const c = TYPE_COLOR[type] ?? "#64748b";
  return {
    borderRadius: 8,
    border: `1px solid ${c}`,
    background: "#fff",
    padding: "6px 10px",
    fontSize: 12,
    color: "#0f172a",
    boxShadow: "0 1px 2px rgba(0,0,0,0.06)",
  };
}

/** A lazily-expanded relationship graph around an object: nodes = objects, edges = links. Click a node
 *  to fan out its declared links; double-click to open its object view. Built on @xyflow/react. */
export function GraphExplorer({ rid }: { rid: string }) {
  const router = useRouter();
  const [nodes, setNodes] = useState<Node<NodeData>[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);
  const expanded = useRef<Set<string>>(new Set());

  const addNeighbors = useCallback(async (id: string, type: string, cx: number, cy: number) => {
    if (expanded.current.has(id)) return;
    expanded.current.add(id);
    const def = OBJECT_TYPES[type];
    if (!def) return;
    const groups = await resolveLinkGroups(def, id);
    const fresh = groups.flatMap((g) => g.rows.map((r) => ({ ...r, targetType: g.targetType })));

    setNodes((prev) => {
      const known = new Set(prev.map((n) => n.id));
      const toAdd = fresh.filter((r) => r.id && !known.has(r.id));
      const ring = toAdd.map((r, i) => {
        const angle = (2 * Math.PI * i) / Math.max(toAdd.length, 1);
        return {
          id: r.id,
          position: { x: cx + Math.cos(angle) * 220, y: cy + Math.sin(angle) * 220 },
          data: { type: r.targetType ?? "", label: r.label },
          style: nodeStyle(r.targetType ?? ""),
        } as Node<NodeData>;
      });
      return [...prev, ...ring];
    });
    setEdges((prev) => {
      const known = new Set(prev.map((e) => e.id));
      const toAdd = fresh
        .filter((r) => r.id)
        .map((r) => ({ id: `${id}->${r.id}`, source: id, target: r.id }))
        .filter((e) => !known.has(e.id));
      return [...prev, ...toAdd];
    });
  }, []);

  // Seed with the center object, then auto-expand one level.
  useEffect(() => {
    const parsed = parseRid(rid);
    if (!parsed) return;
    const def = OBJECT_TYPES[parsed.type];
    let alive = true;
    (async () => {
      let label = parsed.uuid.slice(-8);
      try {
        if (def?.get) {
          const o = await bffGet<Row>(def.get(rid));
          label = def.title(o);
        }
      } catch {
        /* keep the id tail */
      }
      if (!alive) return;
      setNodes([
        {
          id: rid,
          position: { x: 0, y: 0 },
          data: { type: parsed.type, label },
          style: { ...nodeStyle(parsed.type), fontWeight: 600 },
        },
      ]);
      await addNeighbors(rid, parsed.type, 0, 0);
    })();
    return () => {
      alive = false;
    };
  }, [rid, addNeighbors]);

  return (
    <div className="h-[70vh] overflow-hidden rounded-lg border border-slate-200 bg-white">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        fitView
        onNodeClick={(_, n) => addNeighbors(n.id, (n.data as NodeData).type, n.position.x, n.position.y)}
        onNodeDoubleClick={(_, n) => router.push(`/o/${encodeURIComponent(n.id)}`)}
        nodesDraggable
        proOptions={{ hideAttribution: true }}
      >
        <Background />
        <Controls />
      </ReactFlow>
    </div>
  );
}
