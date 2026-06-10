"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { OBJECT_TYPES, type ActionDef, type Row } from "@/lib/ontology/registry";

/** Registry-declared actions for an object, rendered as buttons (object view). The object is passed
 *  as plain data; the registry is resolved here so appliesTo gates work client-side. */
export function ObjectActions({ type, obj }: { type: string; obj: Row }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const def = OBJECT_TYPES[type];
  const actions = (def?.actions ?? []).filter((a) => !a.appliesTo || a.appliesTo(obj));
  if (actions.length === 0) return null;

  const run = async (a: ActionDef) => {
    if (a.confirm && !window.confirm(a.confirm)) return;
    setBusy(true);
    try {
      await mutate(a.method, a.path(obj.id), a.body?.());
      router.refresh();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-wrap gap-2">
      {actions.map((a) => (
        <button
          key={a.key}
          disabled={busy}
          onClick={() => run(a)}
          className={a.danger ? "btn-ghost border-red-300 text-red-700" : "btn-ghost"}
        >
          {a.label}
        </button>
      ))}
    </div>
  );
}
