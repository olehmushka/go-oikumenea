import Link from "next/link";
import { Mono } from "@/components/ui";
import { TypeBadge } from "./TypeBadge";
import type { ObjectTypeDef, Row } from "@/lib/ontology/registry";

/** The header band of an object view/drawer: type badge, title, subtitle, RID, and a graph link. */
export function ObjectHeader({
  def,
  obj,
  action,
}: {
  def: ObjectTypeDef;
  obj: Row;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="min-w-0">
        <div className="mb-1 flex items-center gap-2">
          <TypeBadge type={def.type} />
          {def.subtitle?.(obj) ? (
            <span className="truncate text-xs text-slate-500">{def.subtitle(obj)}</span>
          ) : null}
        </div>
        <h1 className="truncate text-xl font-semibold text-slate-900">{def.title(obj)}</h1>
        <div className="mt-1 flex items-center gap-3">
          <Mono>{obj.id}</Mono>
          <Link href={`/graph/${encodeURIComponent(obj.id)}`} className="text-xs text-indigo-600 hover:underline">
            Open in graph →
          </Link>
        </div>
      </div>
      {action}
    </div>
  );
}
