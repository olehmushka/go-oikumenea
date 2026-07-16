// Maps the backend generic link-traversal response (D-LinkTraversal, review-2026-09 R-27) onto the
// LinksPanel/graph LinkGroup shape. Isomorphic (server + browser); the BACKEND link registry — not
// the web registry's links[] arrays — is now the source of truth for which links an object has.

import type { LinkGroup } from "@/components/ontology/LinksPanel";
import { pickLabel } from "@/lib/i18n";
import { ridTail } from "./rid";

interface RawRow {
  targetRid: string;
  targetType: string;
  targetLabel?: { [k: string]: string } | null;
  attrs?: { [k: string]: string } | null;
}
interface RawGroup {
  linkType: string;
  targetType: string;
  direction: string;
  rows: RawRow[];
}

// member_of -> "Member Of", kin_parent_of -> "Kin Parent Of".
const humanize = (t: string) => t.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());

/** Convert getObjectLinks/searchAround groups into renderable LinkGroups (RID-tail label fallback
 * until server-side neighbor labelers land — a named open seam). */
export function toLinkGroups(groups: RawGroup[] | undefined): LinkGroup[] {
  return (groups ?? []).map((g) => ({
    label: humanize(g.linkType),
    targetType: g.targetType,
    rows: (g.rows ?? []).map((r) => ({
      id: r.targetRid,
      label: (r.targetLabel ? pickLabel(r.targetLabel) : "") || ridTail(r.targetRid),
      sub: r.attrs ? Object.values(r.attrs).filter(Boolean).join(" · ") || undefined : undefined,
    })),
  }));
}
