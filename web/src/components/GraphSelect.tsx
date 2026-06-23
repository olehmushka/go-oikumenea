"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import type { Graph } from "@/lib/api/types";

/**
 * A small dropdown over the tenant graphs (command / operational / directory-only). It submits the
 * graph **code** (not a RID) — the tenant edge/assignment endpoints key on the code and default to
 * `command` when omitted, so the empty option means "command".
 *
 * Uncontrolled (FormData) mode: pass `name`. Controlled mode: pass `value` + `onChange`.
 */
export function GraphSelect({
  name,
  value,
  defaultValue = "",
  onChange,
  includeEmpty = true,
  emptyLabel = "command (default)",
}: {
  name?: string;
  value?: string;
  defaultValue?: string;
  onChange?: (code: string) => void;
  includeEmpty?: boolean;
  emptyLabel?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [graphs, setGraphs] = useState<Graph[]>([]);
  const controlled = value !== undefined;

  useEffect(() => {
    let alive = true;
    api
      .request<{ graphs?: Graph[] }>("GET", "/tenant/v1/graphs")
      .then((d) => {
        if (alive) setGraphs(d?.graphs ?? []);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  return (
    <select
      name={name}
      className="input"
      {...(controlled ? { value } : { defaultValue })}
      onChange={(e) => onChange?.(e.target.value)}
    >
      {includeEmpty ? <option value="">{tr(emptyLabel)}</option> : null}
      {graphs.map((g) => (
        <option key={g.id} value={g.code}>
          {pickLabel(g.name, locale) || g.code}
          {!g.isAuthorityBearing ? " (directory-only)" : ""}
        </option>
      ))}
    </select>
  );
}
