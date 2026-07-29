"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import { Modal } from "@/components/Modal";
import { LocationForm, type Location } from "@/components/LocationForm";

/**
 * Server-query searchable picker (D-WebUI UX). Like LanguagePicker (and unlike EntitySelect's
 * one-page-fetch + client filter), this hits each list endpoint's server-side `query` filter as the
 * user types (debounced) and submits the opaque RID — so the operator never pastes a raw RID. Used by
 * the company workspace for the entities that aren't already loaded into the page (people, other
 * companies, locations).
 *
 * Modes: `name` renders a hidden <input name=…> with the RID (drop-in for FormData forms); `onChange`
 * is the controlled callback. Re-key the element (React `key`) to reset it after a submit.
 */
export type SearchKind = "person" | "company" | "location";

type Result = { id: string; label: string; hint?: string };

type KindConfig = {
  path: (q: string) => string;
  pick: (data: unknown) => unknown[];
  toResult: (item: Record<string, unknown>, locale: string) => Result;
};

const str = (v: unknown): string | undefined => (typeof v === "string" ? v : undefined);
const num = (v: unknown): number | undefined => (typeof v === "number" ? v : undefined);
const map = (v: unknown) => (v && typeof v === "object" ? (v as Record<string, string>) : undefined);

const REGISTRY: Record<SearchKind, KindConfig> = {
  person: {
    path: (q) => `/person/v1/persons?query=${encodeURIComponent(q)}&pageSize=20`,
    pick: (d) => (d as { persons?: unknown[] })?.persons ?? [],
    toResult: (p) => ({
      id: str(p.id) ?? "",
      label: str(p.displayName) || str(p.code) || str(p.id) || "",
      hint: str(p.code),
    }),
  },
  company: {
    path: (q) => `/company/v1/companies?query=${encodeURIComponent(q)}&pageSize=20`,
    pick: (d) => (d as { companies?: unknown[] })?.companies ?? [],
    toResult: (c, locale) => ({
      id: str(c.id) ?? "",
      label: pickLabel(map(c.legalName), locale) || str(c.code) || str(c.id) || "",
      hint: str(c.code),
    }),
  },
  location: {
    path: (q) => `/location/v1/locations?query=${encodeURIComponent(q)}&pageSize=20`,
    pick: (d) => (d as { locations?: unknown[] })?.locations ?? [],
    toResult: (l) => {
      const lat = num(l.latitude);
      const lng = num(l.longitude);
      const coords = lat != null && lng != null ? `${lat.toFixed(4)}, ${lng.toFixed(4)}` : undefined;
      return {
        id: str(l.id) ?? "",
        label: str(l.locality) || str(l.mgrs) || coords || str(l.id) || "",
        hint: str(l.mgrs) || coords,
      };
    },
  },
};

export function SearchSelect({
  kind,
  name,
  defaultValue = "",
  defaultLabel,
  onChange,
  required = false,
  placeholder = "Search…",
  allowCreate = false,
}: {
  kind: SearchKind;
  name?: string;
  /** a preselected RID (e.g. an active URL filter) — shown as a clearable chip on mount. */
  defaultValue?: string;
  /** the human label for defaultValue, when the caller already knows it (an order item's person is
   *  resolved server-side for rendering) — otherwise the chip can only show the RID. */
  defaultLabel?: string;
  /** the label is passed back so a caller can cache rid → label for a later remount */
  onChange?: (id: string, label?: string) => void;
  required?: boolean;
  placeholder?: string;
  // When set (and kind === "location"), show a "＋" button that opens a modal to create a new location
  // inline; on create it is auto-selected here. No-op for other kinds (only locations have a form).
  allowCreate?: boolean;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const cfg = REGISTRY[kind];
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Result[]>([]);
  const [selected, setSelected] = useState<Result | null>(
    defaultValue ? { id: defaultValue, label: defaultLabel || defaultValue } : null,
  );
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);
  const canCreate = allowCreate && kind === "location";

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  useEffect(() => {
    const q = query.trim();
    if (selected || q.length < 1) {
      setResults([]);
      return;
    }
    let alive = true;
    setLoading(true);
    const t = setTimeout(() => {
      api
        .request(`GET`, cfg.path(q))
        .then((d) => {
          if (!alive) return;
          setResults(cfg.pick(d).map((it) => cfg.toResult(it as Record<string, unknown>, locale)));
          setLoading(false);
        })
        .catch(() => alive && setLoading(false));
    }, 200);
    return () => {
      alive = false;
      clearTimeout(t);
    };
  }, [query, selected, cfg, locale]);

  function choose(r: Result) {
    setSelected(r);
    setOpen(false);
    setQuery("");
    onChange?.(r.id, r.label);
  }
  function onLocationCreated(loc: Location) {
    setCreating(false);
    choose(cfg.toResult(loc as unknown as Record<string, unknown>, locale));
  }
  function clear() {
    setSelected(null);
    setQuery("");
    onChange?.("");
  }

  return (
    <div className="relative" ref={boxRef}>
      {name ? <input type="hidden" name={name} value={selected?.id ?? ""} required={required} /> : null}
      {selected ? (
        <div className="flex items-center gap-2">
          <span className="input flex-1 truncate bg-slate-50" title={selected.label}>
            {selected.label}
            {selected.hint && selected.hint !== selected.label ? (
              <span className="ml-2 font-mono text-xs text-slate-400">{selected.hint}</span>
            ) : null}
          </span>
          <button type="button" className="text-xs text-red-600 hover:underline" onClick={clear}>
            {tr("clear")}
          </button>
        </div>
      ) : (
        <div className="flex items-center gap-2">
          <input
            className="input flex-1"
            placeholder={tr(placeholder)}
            value={query}
            required={required}
            autoComplete="off"
            onFocus={() => setOpen(true)}
            onChange={(e) => {
              setQuery(e.target.value);
              setOpen(true);
            }}
          />
          {canCreate ? (
            <button
              type="button"
              className="btn-ghost shrink-0"
              title={tr("Create a location")}
              onClick={() => setCreating(true)}
            >
              ＋
            </button>
          ) : null}
        </div>
      )}
      {open && !selected && (query.trim().length >= 1 || results.length > 0) ? (
        <div className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md border border-slate-200 bg-white shadow-lg">
          {loading ? (
            <div className="px-3 py-2 text-sm text-slate-400">{tr("Searching…")}</div>
          ) : results.length === 0 ? (
            <div className="px-3 py-2 text-sm text-slate-400">{tr("No matches")}</div>
          ) : (
            results.map((r) => (
              <button
                key={r.id}
                type="button"
                className="flex w-full items-center justify-between gap-2 px-3 py-1.5 text-left text-sm hover:bg-indigo-50"
                onClick={() => choose(r)}
              >
                <span className="truncate text-slate-800">{r.label}</span>
                {r.hint ? <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">{r.hint}</span> : null}
              </button>
            ))
          )}
        </div>
      ) : null}
      {canCreate ? (
        <Modal open={creating} title={tr("Create a location")} onClose={() => setCreating(false)}>
          <LocationForm onCreated={onLocationCreated} submitLabel={tr("Create and select")} />
        </Modal>
      ) : null}
    </div>
  );
}
