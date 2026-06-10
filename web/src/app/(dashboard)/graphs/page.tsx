import { apiGet } from "@/lib/api/server";
import { Card, ErrorNotice, PageHeader } from "@/components/ui";
import { GraphManager } from "@/components/GraphManager";
import { ClosureTools } from "@/components/ClosureTools";
import type { GraphList } from "@/lib/api/types";

// Graph administration (tenant): named hierarchies (CRUD) + the transitive-closure maintenance the PDP
// depends on. Browsing graphs lives in the object explorer (/explore/graph); this is the admin surface.
export default async function GraphsPage() {
  let graphs: GraphList | null = null;
  let error: unknown = null;
  try {
    graphs = await apiGet<GraphList>("/tenant/v1/graphs");
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title="Graph admin"
        description="Named unit hierarchies and the transitive closure that feeds the PDP."
      />
      {error ? <ErrorNotice error={error} /> : null}
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <h2 className="mb-3 text-sm font-semibold text-slate-900">Graphs</h2>
          <GraphManager graphs={graphs?.graphs ?? []} />
        </Card>
        <Card>
          <h2 className="mb-3 text-sm font-semibold text-slate-900">Closure</h2>
          <p className="mb-3 text-xs text-slate-500">
            Rebuild or verify the materialized transitive-closure table (descendant/ancestor reach).
          </p>
          <ClosureTools />
        </Card>
      </div>
    </div>
  );
}
