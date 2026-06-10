import { typeDef } from "@/lib/ontology/registry";
import { ridKind } from "@/lib/ontology/rid";

const KIND_STYLE: Record<string, string> = {
  object: "bg-indigo-50 text-indigo-700 ring-indigo-200",
  link: "bg-amber-50 text-amber-800 ring-amber-200",
  action: "bg-emerald-50 text-emerald-700 ring-emerald-200",
};

/** A compact badge for an ontology type token (Object / Link / Action), label from the registry. */
export function TypeBadge({ type, className = "" }: { type: string; className?: string }) {
  const def = typeDef(type);
  const kind = def?.kind ?? ridKind(type);
  const label = def?.label ?? type;
  return (
    <span
      className={`inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium ring-1 ring-inset ${KIND_STYLE[kind]} ${className}`}
      title={`${kind}: ${type}`}
    >
      {label}
    </span>
  );
}
