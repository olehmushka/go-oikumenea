"use client";

import { useMemo } from "react";
import { useRouter } from "next/navigation";
import { useTg } from "@/lib/locale";
import type { ActionType } from "@/lib/api/types";

/** Audit action filter, backed by the registered action-type catalog (D-ActionTypes, R-29) rather than
 * free text: a grouped-by-service dropdown of every valid `audit_log.action` code, navigating to
 * `/audit?action=<code>`. A milestone that adds an action code surfaces here automatically. */
export function ActionFilter({ current, actions }: { current?: string; actions: ActionType[] }) {
  const router = useRouter();
  const tr = useTg();

  const byService = useMemo(() => {
    const m = new Map<string, ActionType[]>();
    for (const a of [...actions].sort((x, y) => x.code.localeCompare(y.code))) {
      (m.get(a.service) ?? m.set(a.service, []).get(a.service)!).push(a);
    }
    return [...m.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [actions]);

  return (
    <div className="card p-4">
      <label className="label" htmlFor="action-filter">{tr("Filter by action")}</label>
      <select
        id="action-filter"
        className="input"
        value={current ?? ""}
        onChange={(e) =>
          router.push(e.target.value ? `/audit?action=${encodeURIComponent(e.target.value)}` : "/audit")
        }
      >
        <option value="">{tr("All actions")}</option>
        {byService.map(([service, list]) => (
          <optgroup key={service} label={service}>
            {list.map((a) => (
              <option key={a.code} value={a.code}>
                {a.code}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </div>
  );
}
