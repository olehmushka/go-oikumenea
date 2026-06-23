"use client";

// Browser-side helper for the data-driven ontology layer, through the single SDK client (api), which
// targets the BFF proxy (the proxy injects the bearer; the browser never holds a token).

import type { LinkGroup } from "@/components/ontology/LinksPanel";
import type { ObjectTypeDef } from "@/lib/ontology/registry";
import { api } from "./client";

/** Resolve a type's declared link collections for one object into renderable groups. */
export async function resolveLinkGroups(def: ObjectTypeDef, id: string): Promise<LinkGroup[]> {
  const defs = (def.links ?? []).filter((l) => l.path(id) !== "");
  const groups = await Promise.all(
    defs.map(async (l): Promise<LinkGroup> => {
      try {
        const res = await api.request("GET", l.path(id));
        return { label: l.label, targetType: l.targetType, rows: l.parse(res, id) };
      } catch {
        return { label: l.label, targetType: l.targetType, rows: [] };
      }
    }),
  );
  return groups;
}
