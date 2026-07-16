"use client";

// Browser-side helper for the data-driven ontology layer, through the single SDK client (api), which
// targets the BFF proxy (the proxy injects the bearer; the browser never holds a token).

import type { LinkGroup } from "@/components/ontology/LinksPanel";
import type { ObjectTypeDef } from "@/lib/ontology/registry";
import { toLinkGroups } from "@/lib/ontology/links";
import { ridTail } from "@/lib/ontology/rid";
import { pickLabel } from "@/lib/i18n";
import { api } from "./client";

/** Resolve one object's links via the generic backend traversal API (D-LinkTraversal, R-27): ONE
 * request (previously ~19, one per declared collection), errors surface instead of rendering as
 * empty groups, and a link table added by a new milestone shows up here without editing registry.ts.
 * Grouped/dossier shape — used by the object view & drawer. */
export async function resolveLinkGroups(_def: ObjectTypeDef, id: string): Promise<LinkGroup[]> {
  const res = await api.links.getObjectLinks(id, undefined, 200);
  return toLinkGroups(res.groups);
}

/** A flat graph neighbor of an object. */
export interface Neighbor {
  id: string;
  label: string;
  targetType: string;
}

/** Resolve one object's depth-1 neighbors via the flat search-around traversal (D-LinkTraversal, R-27) —
 * the graph-explorer shape (vs. resolveLinkGroups' grouped dossier shape). Works for ANY object type
 * straight from its RID, so a link table added by a new milestone expands in the graph without touching
 * the web registry. */
export async function resolveNeighbors(id: string): Promise<Neighbor[]> {
  const res = await api.links.searchAround(id, undefined, 200);
  return (res.neighbors ?? []).map((n) => ({
    id: n.targetRid,
    label: (n.targetLabel ? pickLabel(n.targetLabel) : "") || ridTail(n.targetRid),
    targetType: n.targetType,
  }));
}
