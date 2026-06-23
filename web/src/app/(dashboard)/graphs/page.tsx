import { oikumenea } from "@/lib/api/server";
import { Card, ErrorNotice, PageHeader } from "@/components/ui";
import { GraphManager } from "@/components/GraphManager";
import { ClosureTools } from "@/components/ClosureTools";
import { T } from "@/components/T";
import type { GraphList } from "@/lib/api/types";

// Graph administration (tenant): named hierarchies (CRUD) + the transitive-closure maintenance the PDP
// depends on. Browsing graphs lives in the object explorer (/explore/graph); this is the admin surface.
export default async function GraphsPage() {
  let graphs: GraphList | null = null;
  let error: unknown = null;
  try {
    graphs = await oikumenea().then((ok) => ok.tenant.listGraphs());
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title={<T>Graph admin</T>}
        description={<T>Named unit hierarchies and the transitive closure that feeds the PDP.</T>}
      />
      {error ? <ErrorNotice error={error} /> : null}
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>Graphs</T></h2>
          <GraphManager graphs={graphs?.graphs ?? []} />
        </Card>
        <Card>
          <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>Closure</T></h2>
          <p className="mb-3 text-xs text-slate-500">
            <T>Rebuild or verify the materialized transitive-closure table (descendant/ancestor reach).</T>
          </p>
          <ClosureTools />
        </Card>
      </div>
    </div>
  );
}
