import Link from "next/link";
import { PageHeader } from "@/components/ui";
import { GraphExplorer } from "@/components/ontology/GraphExplorer";
import { TypeBadge } from "@/components/ontology/TypeBadge";
import { parseRid } from "@/lib/ontology/rid";

// A visual relationship graph around any object — traverse the unit DAG, a person's memberships/orders,
// etc. Click a node to expand its links; double-click to open its object view.
export default async function GraphPage({ params }: { params: Promise<{ rid: string }> }) {
  const { rid: raw } = await params;
  const rid = decodeURIComponent(raw);
  const parsed = parseRid(rid);

  return (
    <div>
      <PageHeader
        title="Relationship graph"
        description="Click a node to fan out its links; double-click to open it."
        action={
          <div className="flex items-center gap-3">
            {parsed ? <TypeBadge type={parsed.type} /> : null}
            <Link href={`/o/${encodeURIComponent(rid)}`} className="btn-ghost">
              Object view →
            </Link>
          </div>
        }
      />
      {parsed ? (
        <GraphExplorer rid={rid} />
      ) : (
        <p className="text-sm text-red-600">Not a valid RID.</p>
      )}
    </div>
  );
}
