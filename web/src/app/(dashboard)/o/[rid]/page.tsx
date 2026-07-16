import Link from "next/link";
import { redirect } from "next/navigation";
import { oikumenea } from "@/lib/api/server";
import { Card, ErrorNotice, PageHeader } from "@/components/ui";
import { ObjectHeader } from "@/components/ontology/ObjectHeader";
import { ObjectActions } from "@/components/ontology/ObjectActions";
import { PropertyList } from "@/components/ontology/PropertyList";
import { LinksPanel, type LinkGroup } from "@/components/ontology/LinksPanel";
import { HistoryPanel } from "@/components/ontology/HistoryPanel";
import { ActionRunner } from "@/components/ontology/ActionRunner";
import { RecordVisit } from "@/components/ontology/RecordVisit";
import type { ActionType } from "@/lib/api/types";
import { OBJECT_TYPES, type Row } from "@/lib/ontology/registry";
import { toLinkGroups } from "@/lib/ontology/links";
import { parseRid } from "@/lib/ontology/rid";
import { T } from "@/components/T";

// Types that already have a richer bespoke editor page — land traversal there; the generic view below
// covers every other type (role, assignment, position, document, catalogs, graph…).
const BESPOKE_ROUTE: Record<string, (id: string) => string> = {
  person: (id) => `/persons/${id}`,
  unit: (id) => `/units/${id}`,
  order: (id) => `/orders/${id}`,
};

// The universal object view — one page for any object, keyed by its self-describing RID. Generalizes
// the bespoke detail pages: header + properties + grouped, traversable Links panel.
export default async function ObjectPage({ params }: { params: Promise<{ rid: string }> }) {
  const { rid: raw } = await params;
  const rid = decodeURIComponent(raw);
  const parsed = parseRid(rid);
  if (parsed && BESPOKE_ROUTE[parsed.type]) redirect(BESPOKE_ROUTE[parsed.type](rid));
  const def = parsed ? OBJECT_TYPES[parsed.type] : undefined;

  if (!parsed || !def || !def.get) {
    return (
      <div>
        <PageHeader title="Object" />
        <ErrorNotice
          error={new Error(
            !parsed
              ? "Not a valid RID."
              : `No detail view registered for type "${parsed.type}".`,
          )}
        />
      </div>
    );
  }

  let obj: Row | null = null;
  let error: unknown = null;
  let groups: LinkGroup[] = [];
  let actions: ActionType[] = [];
  try {
    const ok = await oikumenea();
    obj = await ok.request<Row>("GET", def.get(rid));
    // One generic traversal call replaces the former per-collection fan-out (D-LinkTraversal, R-27).
    const linked = await ok.links.getObjectLinks(rid, undefined, 200);
    groups = toLinkGroups(linked.groups);
    // The type's registered actions + parameter schemas (D-ActionTypes, R-29) — enumerated from the
    // endpoint, not registry.ts. Read-only discoverability; invocation stays on the bespoke forms.
    const cat = await ok.audit.listActionTypes().catch(() => [] as ActionType[]);
    actions = cat.filter((a) => a.targetType === def.type);
  } catch (e) {
    error = e;
  }

  if (error || !obj) {
    return (
      <div>
        <PageHeader title={<T>{def.label}</T>} />
        <ErrorNotice error={error} />
      </div>
    );
  }

  return (
    <div>
      <RecordVisit id={obj.id} type={def.type} label={def.title(obj)} />
      <div className="mb-4 flex items-center justify-between">
        <Link href={`/explore/${def.type}`} className="text-sm text-indigo-600 hover:underline">
          ← <T>All</T> <span className="lowercase"><T>{def.labelPlural}</T></span>
        </Link>
      </div>

      <Card>
        <ObjectHeader def={def} obj={obj} action={<ObjectActions type={def.type} obj={obj} />} />
      </Card>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card>
          <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>Properties</T></h2>
          <PropertyList def={def} obj={obj} />
        </Card>
        <Card>
          <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>Links</T></h2>
          <LinksPanel groups={groups} />
        </Card>
      </div>

      {actions.length > 0 ? (
        <Card className="mt-4">
          <h2 className="mb-1 text-sm font-semibold text-slate-900"><T>Actions</T></h2>
          <p className="mb-3 text-xs text-slate-500">
            <T>Registered actions for this type (D-ActionTypes, R-29), each bound to its endpoint (D-ActionInvocation, R-33). Object-level actions with a simple body can be run inline; structured or sub-resource actions run from the object's dedicated form.</T>
          </p>
          <ActionRunner actions={actions} rid={rid} />
        </Card>
      ) : null}

      <Card className="mt-4">
        <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>History</T></h2>
        <HistoryPanel rid={rid} />
      </Card>
    </div>
  );
}
