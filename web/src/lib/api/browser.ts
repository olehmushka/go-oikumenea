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
  /** 1 = direct neighbor, 2 = second hop (depth-2 search-around); 1 for depth-1 results. */
  hop?: number;
  /** For a hop-2 neighbor, the intermediate hop-1 node RID it was reached through. */
  via?: string;
}

/** Resolve one object's neighbors via the flat search-around traversal (D-LinkTraversal, R-27) — the
 * graph-explorer shape (vs. resolveLinkGroups' grouped dossier shape). Works for ANY object type
 * straight from its RID, so a link table added by a new milestone expands in the graph without touching
 * the web registry. depth=1 (default) is the direct neighborhood; depth=2 additionally returns each
 * neighbor's own neighbors (rows carry hop / via) in one request. */
export async function resolveNeighbors(id: string, depth = 1): Promise<Neighbor[]> {
  const res = await api.links.searchAround(id, depth, undefined, 200);
  return (res.neighbors ?? []).map((n) => ({
    id: n.targetRid,
    label: (n.targetLabel ? pickLabel(n.targetLabel) : "") || ridTail(n.targetRid),
    targetType: n.targetType,
    hop: n.hop ?? 1,
    via: n.viaRid ?? undefined,
  }));
}
