"use client";

// A plain <select> for a catalog that only exists BELOW something else — unit kinds under a domain
// (the listUnitKinds `domain` arg is required), positions under a unit (there is no global list).
//
// `path` is null until the scoping value is chosen, and a null path renders the control disabled with
// an honest placeholder rather than an empty dropdown that looks broken — the idiom UnitSelect
// already uses for its org→unit cascade ("pick an organization first").

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";

type Option = { id: string; label: string; hint?: string };

const str = (v: unknown): string | undefined => (typeof v === "string" ? v : undefined);
const map = (v: unknown) => (v && typeof v === "object" ? (v as Record<string, string>) : undefined);

export function ScopedSelect({
  path,
  pick,
  value,
  onChange,
  emptyLabel = "Any",
  unscopedLabel,
  className = "input max-w-xs",
}: {
  /** the list endpoint incl. its scoping query arg, or null while the scope is unchosen */
  path: string | null;
  /** pull the rows out of the envelope (e.g. d.unitKinds, d.positions) */
  pick: (data: unknown) => unknown[];
  value: string;
  onChange: (id: string) => void;
  emptyLabel?: string;
  /** what to say while `path` is null, e.g. "pick a domain first" */
  unscopedLabel: string;
  className?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [options, setOptions] = useState<Option[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!path) {
      setOptions([]);
      return;
    }
    let alive = true;
    setLoading(true);
    api
      .request("GET", path)
      .then((d) => {
        if (!alive) return;
        setOptions(
          pick(d).map((it) => {
            const r = it as Record<string, unknown>;
            return {
              id: str(r.id) ?? "",
              // `title` as well as `name`: a position's translatable label is `title`, so reading
              // only `name` silently fell back to the code for every position.
              label:
                pickLabel(map(r.name), locale) ||
                pickLabel(map(r.title), locale) ||
                str(r.code) ||
                str(r.id) ||
                "",
              hint: str(r.code),
            };
          }),
        );
      })
      .catch(() => {
        if (alive) setOptions([]);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
    // `pick` is a stable literal at every call site; re-running on identity churn would refetch per render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, locale]);

  return (
    <select
      className={className}
      value={value}
      disabled={!path}
      onChange={(e) => onChange(e.target.value)}
    >
      <option value="">
        {!path ? tr(unscopedLabel) : loading ? tr("Loading…") : tr(emptyLabel)}
      </option>
      {options.map((o) => (
        <option key={o.id} value={o.id}>
          {o.label}
          {o.hint && o.hint !== o.label ? ` (${o.hint})` : ""}
        </option>
      ))}
    </select>
  );
}
