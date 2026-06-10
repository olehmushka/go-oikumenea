"use client";

// Browser-side reads through the BFF proxy (the proxy injects the bearer; the browser never holds a
// token). Mirrors apiGet on the server. Used by the command palette, the detail drawer, and the graph.

import type { LinkGroup } from "@/components/ontology/LinksPanel";
import type { ObjectTypeDef } from "@/lib/ontology/registry";

export async function bffGet<T = unknown>(path: string, search = ""): Promise<T> {
  const res = await fetch(`/api/oikumenea${path}${search}`, {
    headers: { Accept: "application/json" },
  });
  if (!res.ok) throw new Error(`Request failed (${res.status})`);
  return (await res.json()) as T;
}

/** Resolve a type's declared link collections for one object into renderable groups. */
export async function resolveLinkGroups(def: ObjectTypeDef, id: string): Promise<LinkGroup[]> {
  const defs = (def.links ?? []).filter((l) => l.path(id) !== "");
  const groups = await Promise.all(
    defs.map(async (l): Promise<LinkGroup> => {
      try {
        const res = await bffGet(l.path(id));
        return { label: l.label, targetType: l.targetType, rows: l.parse(res, id) };
      } catch {
        return { label: l.label, targetType: l.targetType, rows: [] };
      }
    }),
  );
  return groups;
}
