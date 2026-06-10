import Link from "next/link";
import { Mono, Pill } from "@/components/ui";
import { TypeBadge } from "./TypeBadge";
import type { LinkRow } from "@/lib/ontology/registry";

export interface LinkGroup {
  label: string;
  targetType?: string;
  rows: LinkRow[];
}

/** A grouped panel of related objects you can traverse (each row → /o/<id>). Data pre-resolved. */
export function LinksPanel({ groups }: { groups: LinkGroup[] }) {
  const nonEmpty = groups.filter((g) => g.rows.length > 0);
  if (nonEmpty.length === 0)
    return <p className="text-sm text-slate-400">No links.</p>;
  return (
    <div className="space-y-5">
      {nonEmpty.map((g) => (
        <div key={g.label}>
          <div className="mb-2 flex items-center gap-2">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              {g.label}
            </h3>
            <span className="text-xs text-slate-400">{g.rows.length}</span>
            {g.targetType ? <TypeBadge type={g.targetType} /> : null}
          </div>
          <ul className="divide-y divide-slate-100 rounded-md border border-slate-200">
            {g.rows.map((r, i) => (
              <li key={`${r.id}-${i}`} className="flex items-center justify-between gap-2 px-3 py-1.5 text-sm hover:bg-slate-50">
                <Link href={`/o/${encodeURIComponent(r.id)}`} className="flex items-center gap-2 text-indigo-600 hover:underline">
                  <Mono>{r.label}</Mono>
                </Link>
                {r.sub ? <Pill tone={r.tone ?? "slate"}>{r.sub}</Pill> : null}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
