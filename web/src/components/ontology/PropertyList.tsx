import type { ObjectTypeDef, Row } from "@/lib/ontology/registry";
import { Value } from "./Value";

/** Renders an object's registry-declared properties as a definition list. */
export function PropertyList({ def, obj }: { def: ObjectTypeDef; obj: Row }) {
  const props = def.properties ?? [];
  if (props.length === 0)
    return <p className="text-sm text-slate-400">No properties declared.</p>;
  return (
    <dl className="space-y-2 text-sm">
      {props.map((p) => (
        <div key={p.label} className="flex justify-between gap-4">
          <dt className="text-slate-500">{p.label}</dt>
          <dd className="text-right text-slate-800">
            <Value value={p.value(obj)} render={p.render} tone={p.tone?.(obj)} />
          </dd>
        </div>
      ))}
    </dl>
  );
}
