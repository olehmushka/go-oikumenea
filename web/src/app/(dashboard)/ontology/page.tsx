import Link from "next/link";
import { PageHeader } from "@/components/ui";
import { TypeBadge } from "@/components/ontology/TypeBadge";
import { OBJECT_TYPES, type ObjectTypeDef } from "@/lib/ontology/registry";

// The human-facing mirror of D-Ontology (docs/ontology-mapping.md): the Object/Link type registry the
// whole console is built on. Each type links to its explorer (when globally listable).
export default function OntologyPage() {
  const defs = Object.values(OBJECT_TYPES);
  const objects = defs.filter((d) => d.kind === "object");
  const links = defs.filter((d) => d.kind === "link");

  return (
    <div>
      <PageHeader
        title="Ontology"
        description="Every entity is a typed Object or reified Link, keyed by a self-describing RID. This is the registry that powers search, the explorer, and link traversal."
      />
      <Section title="Objects" defs={objects} />
      <Section title="Links" defs={links} />
    </div>
  );
}

function Section({ title, defs }: { title: string; defs: ObjectTypeDef[] }) {
  if (defs.length === 0) return null;
  return (
    <div className="mb-8">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">{title}</h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {defs.map((d) => {
          const card = (
            <div className="card h-full p-4 transition-colors hover:border-indigo-300">
              <div className="mb-2 flex items-center justify-between gap-2">
                <span className="font-medium text-slate-900">{d.label}</span>
                <TypeBadge type={d.type} />
              </div>
              <p className="text-xs text-slate-500">{d.blurb}</p>
              <div className="mt-3 flex items-center gap-2 text-xs text-slate-400">
                <span className="font-mono">{d.type}</span>
                <span>·</span>
                <span>{d.module}</span>
                {d.list ? <span className="ml-auto text-indigo-600">Browse →</span> : null}
              </div>
            </div>
          );
          return d.list ? (
            <Link key={d.type} href={`/explore/${d.type}`}>
              {card}
            </Link>
          ) : (
            <div key={d.type}>{card}</div>
          );
        })}
      </div>
    </div>
  );
}
