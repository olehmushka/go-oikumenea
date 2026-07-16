import { T } from "@/components/T";
import type { ActionParam } from "@/lib/api/types";

/** Read-only render of an action's parameter schema (D-ActionTypes, R-29), single-sourced from the
 * Conjure request type. Descriptive/discoverability only — not an invocation form. */
export function ActionParamsList({ params }: { params?: ActionParam[] }) {
  if (!params || params.length === 0)
    return <span className="text-xs text-slate-400"><T>no parameters</T></span>;
  return (
    <ul className="space-y-0.5">
      {params.map((p) => (
        <li key={p.name} className="text-xs">
          <code className="font-mono text-slate-700">{p.name}</code>
          <span className="text-slate-400"> : {p.type}</span>
          {p.required ? <span className="text-amber-600" title="required"> *</span> : null}
          {p.docs ? <span className="text-slate-400"> — {p.docs}</span> : null}
        </li>
      ))}
    </ul>
  );
}
