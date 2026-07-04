"use client";

// Org-scoped unit picker (D-WebUI UX). The tenant listUnits endpoint REQUIRES an org RID and has no
// server-side text search (M40), so this cascades: pick an organization, then type-to-filter its units
// (client-side over one page of ≤200). Emits the selected unit RID via onChange — a drop-in replacement
// for the raw "org unit RID" text inputs. Controlled by the parent through onChange only.

import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Org = { id: string; code: string; name: LocaleMap };
type Unit = { id: string; code?: string; name: LocaleMap };

export function UnitSelect({
  onChange,
  placeholder = "search a unit…",
}: {
  onChange: (unitId: string) => void;
  placeholder?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [orgs, setOrgs] = useState<Org[]>([]);
  const [orgId, setOrgId] = useState("");
  const [units, setUnits] = useState<Unit[]>([]);
  const [loadingUnits, setLoadingUnits] = useState(false);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Unit | null>(null);
  const [open, setOpen] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.tenant
      .listOrganizations()
      .then((r) => setOrgs((r.organizations ?? []) as unknown as Org[]))
      .catch(() => setOrgs([]));
  }, []);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  // Reload the org's units (and reset the current pick) whenever the organization changes.
  useEffect(() => {
    setSelected(null);
    setQuery("");
    onChange("");
    if (!orgId) {
      setUnits([]);
      return;
    }
    setLoadingUnits(true);
    api.tenant
      .listUnits(orgId, undefined, undefined, undefined, undefined, undefined, undefined, 200)
      .then((r) => setUnits((r.units ?? []) as unknown as Unit[]))
      .catch(() => setUnits([]))
      .finally(() => setLoadingUnits(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgId]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const base = q
      ? units.filter(
          (u) =>
            (pickLabel(u.name, locale) || "").toLowerCase().includes(q) ||
            (u.code ?? "").toLowerCase().includes(q),
        )
      : units;
    return base.slice(0, 50);
  }, [units, query, locale]);

  function choose(u: Unit) {
    setSelected(u);
    setOpen(false);
    setQuery("");
    onChange(u.id);
  }
  function clear() {
    setSelected(null);
    setQuery("");
    onChange("");
  }

  return (
    <div className="flex gap-2">
      <select className="input w-40 shrink-0" value={orgId} onChange={(e) => setOrgId(e.target.value)}>
        <option value="">{tr("— organization —")}</option>
        {orgs.map((o) => (
          <option key={o.id} value={o.id}>
            {o.code || pickLabel(o.name, locale)}
          </option>
        ))}
      </select>
      <div className="relative flex-1" ref={boxRef}>
        {selected ? (
          <div className="flex items-center gap-2">
            <span className="input flex-1 truncate bg-slate-50" title={pickLabel(selected.name, locale)}>
              {pickLabel(selected.name, locale) || selected.code}
              {selected.code ? <span className="ml-2 font-mono text-xs text-slate-400">{selected.code}</span> : null}
            </span>
            <button type="button" className="text-xs text-red-600 hover:underline" onClick={clear}>
              {tr("clear")}
            </button>
          </div>
        ) : (
          <input
            className="input w-full"
            placeholder={!orgId ? tr("pick an organization first") : loadingUnits ? tr("Loading…") : tr(placeholder)}
            value={query}
            disabled={!orgId}
            autoComplete="off"
            onFocus={() => setOpen(true)}
            onChange={(e) => {
              setQuery(e.target.value);
              setOpen(true);
            }}
          />
        )}
        {open && orgId && !selected ? (
          <div className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md border border-slate-200 bg-white shadow-lg">
            {loadingUnits ? (
              <div className="px-3 py-2 text-sm text-slate-400">{tr("Loading…")}</div>
            ) : filtered.length === 0 ? (
              <div className="px-3 py-2 text-sm text-slate-400">{tr("No units")}</div>
            ) : (
              filtered.map((u) => (
                <button
                  key={u.id}
                  type="button"
                  className="flex w-full items-center justify-between gap-2 px-3 py-1.5 text-left text-sm hover:bg-indigo-50"
                  onClick={() => choose(u)}
                >
                  <span className="truncate text-slate-800">{pickLabel(u.name, locale) || u.code || u.id}</span>
                  {u.code ? <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">{u.code}</span> : null}
                </button>
              ))
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}
