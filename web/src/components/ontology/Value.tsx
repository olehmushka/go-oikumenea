import { Mono, Pill } from "@/components/ui";
import type { Tone } from "@/lib/ontology/registry";

/** Renders a registry cell/property value honouring its render hint (mono / pill / text). */
export function Value({
  value,
  render,
  tone = "slate",
}: {
  value: string | number | undefined;
  render?: "mono" | "pill" | "text";
  tone?: Tone;
}) {
  if (value === undefined || value === "" || value === null)
    return <span className="text-slate-300">—</span>;
  if (render === "mono") return <Mono>{value}</Mono>;
  if (render === "pill") return <Pill tone={tone}>{value}</Pill>;
  return <>{value}</>;
}
