"use client";

import { useEffect } from "react";
import { pushRecent } from "@/lib/ontology/recents";

/** Fire-and-forget: records an object visit into per-browser recents (for Overview + palette). */
export function RecordVisit({ id, type, label }: { id: string; type: string; label: string }) {
  useEffect(() => {
    pushRecent({ id, type, label });
  }, [id, type, label]);
  return null;
}
