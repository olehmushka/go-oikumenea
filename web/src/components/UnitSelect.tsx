"use client";

// Org-scoped unit picker (D-WebUI UX). The tenant listUnits endpoint REQUIRES an org RID (M40), so
// this cascades: pick an organization, then search its units. It is the correct picker for units
// everywhere a flat list would 400 (a fully-unscoped `/tenant/v1/units` is rejected) — EntitySelect
// delegates kind="unit" here.
//
// The text search is SERVER-SIDE (`&query=`, migration 0022). It used to filter one page of ≤200 in
// the browser, which silently made every unit past that page unselectable in a large org — the same
// defect fixed for the person picker, and invisible in the same way: the dropdown reports
// "No matches" for units that plainly exist.
//
// Integration modes (mirrors EntitySelect):
//  - `name`     → renders a hidden <input name=…> holding the RID, so FormData-based forms work.
//  - `onChange` → controlled callback with the selected RID (lookup filters / EdgeManager).

import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Org = { id: string; code: string; name: LocaleMap };
type Unit = { id: string; code?: string; name: LocaleMap };

export function UnitSelect({
  onChange,
  name,
  defaultValue = "",
  defaultLabel,
  required = false,
  allowEmpty = false,
  placeholder = "search a unit…",
}: {
  /** the label is passed back so a caller can cache rid → label for a later remount */
  onChange?: (unitId: string, label?: string) => void;
  name?: string;
  defaultValue?: string;
  /** the human label for defaultValue, when the caller already knows it — otherwise a preselected
   *  unit can only show its RID until the operator re-picks it. */
  defaultLabel?: string;
  required?: boolean;
  allowEmpty?: boolean;
  placeholder?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [orgs, setOrgs] = useState<Org[]>([]);
  const [orgId, setOrgId] = useState("");
  const [units, setUnits] = useState<Unit[]>([]);
  const [loadingUnits, setLoadingUnits] = useState(false);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [selected, setSelected] = useState<Unit | null>(null);
  // The emitted RID. Seeded from defaultValue so a pre-set value (e.g. an active filter) survives mount.
  const [value, setValue] = useState(defaultValue);
  const [open, setOpen] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);
  const orgTouched = useRef(false);

  const emit = (id: string, label?: string) => {
    setValue(id);
    onChange?.(id, label);
  };

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

  // Debounced copy of the typed query, so a name is one request rather than one per keystroke.
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query.trim()), 200);
    return () => clearTimeout(t);
  }, [query]);

  // Changing the ORGANIZATION resets the pick — a unit RID from the previous org is meaningless here.
  // This is deliberately SEPARATE from the fetch below: folding it in would also fire on every
  // keystroke (the fetch now depends on the query too) and clear the user's selection as they type.
  // Skip the initial mount so a controlled defaultValue isn't clobbered by an onChange("").
  useEffect(() => {
    if (!orgTouched.current) return;
    setSelected(null);
    setQuery("");
    emit("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgId]);

  // Fetch the org's units, re-running when the debounced search term changes.
  useEffect(() => {
    if (!orgId) {
      setUnits([]);
      return;
    }
    setLoadingUnits(true);
    // The generic escape hatch, NOT the positional `listUnits(...)`: every facet arg added to the
    // middle of that signature (M56's six, M57's levelMin/levelMax) silently shifted `200` onto
    // whatever now sits in the 11th slot. A query string names what it means and cannot shift.
    const q = debouncedQuery ? `&query=${encodeURIComponent(debouncedQuery)}` : "";
    api
      .request("GET", "/tenant/v1/units", { query: `?org=${encodeURIComponent(orgId)}&pageSize=200${q}` })
      .then((r) => setUnits(((r as { units?: unknown[] }).units ?? []) as unknown as Unit[]))
      .catch(() => setUnits([]))
      .finally(() => setLoadingUnits(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orgId, debouncedQuery]);

  // Server-filtered, so the rows are NOT re-filtered here: the server matches the unit's code as well
  // as its name, and a client-side test against the localized label alone would discard correct hits
  // the user has no other way to reach.
  const filtered = useMemo(() => units.slice(0, 50), [units]);

  function choose(u: Unit) {
    setSelected(u);
    setOpen(false);
    setQuery("");
    emit(u.id, pickLabel(u.name, locale) || u.code || u.id);
  }
  function clear() {
    setSelected(null);
    setQuery("");
    emit("");
  }

  // A value with no loaded Unit object (a preselected defaultValue) still shows a clearable chip.
  const hasChip = selected || (value && !orgTouched.current);
  const chipLabel = selected ? pickLabel(selected.name, locale) || selected.code : defaultLabel || value;

  return (
    <div className="flex gap-2">
      {name ? <input type="hidden" name={name} value={value} /> : null}
      <select
        className="input w-40 shrink-0"
        value={orgId}
        onChange={(e) => {
          orgTouched.current = true;
          setOrgId(e.target.value);
        }}
      >
        <option value="">{tr("— organization —")}</option>
        {orgs.map((o) => (
          <option key={o.id} value={o.id}>
            {o.code || pickLabel(o.name, locale)}
          </option>
        ))}
      </select>
      <div className="relative flex-1" ref={boxRef}>
        {hasChip ? (
          <div className="flex items-center gap-2">
            <span className="input flex-1 truncate bg-slate-50" title={chipLabel}>
              {chipLabel}
              {selected?.code ? <span className="ml-2 font-mono text-xs text-slate-400">{selected.code}</span> : null}
            </span>
            {(allowEmpty || selected) && (
              <button type="button" className="text-xs text-red-600 hover:underline" onClick={clear}>
                {tr("clear")}
              </button>
            )}
          </div>
        ) : (
          <input
            className="input w-full"
            placeholder={!orgId ? tr("pick an organization first") : loadingUnits ? tr("Loading…") : tr(placeholder)}
            value={query}
            disabled={!orgId}
            required={required && !value}
            autoComplete="off"
            onFocus={() => setOpen(true)}
            onChange={(e) => {
              setQuery(e.target.value);
              setOpen(true);
            }}
          />
        )}
        {open && orgId && !hasChip ? (
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
