"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useLocale } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";

/**
 * Searchable entity picker (D-WebUI UX): replaces free-text RID inputs with a type-to-filter
 * dropdown that shows human labels (name/code) and submits the opaque RID. It fetches one page
 * of the relevant list endpoint through the BFF proxy and filters client-side — fine for the
 * admin/dev scale these directories have (the list endpoints have no server-side search param).
 *
 * Two integration modes:
 *  - `name`     → renders a hidden <input name=…> holding the RID, so existing FormData-based
 *                 forms keep working unchanged.
 *  - `onChange` → controlled callback with the selected RID (used by lookup filters / EdgeManager).
 */
export type EntityKind = "person" | "unit" | "role" | "orderType";

type Option = { id: string; label: string; hint?: string };

type KindConfig = {
  path: string; // includes any query string
  pick: (data: unknown) => unknown[];
  toOption: (item: Record<string, unknown>, locale: string) => Option;
};

const str = (v: unknown): string | undefined => (typeof v === "string" ? v : undefined);
const map = (v: unknown) => (v && typeof v === "object" ? (v as Record<string, string>) : undefined);

const REGISTRY: Record<EntityKind, KindConfig> = {
  person: {
    path: "/person/v1/persons?pageSize=200",
    pick: (d) => (d as { persons?: unknown[] })?.persons ?? [],
    toOption: (p) => ({
      id: str(p.id) ?? "",
      label: str(p.displayName) || str(p.code) || str(p.id) || "",
      hint: str(p.code),
    }),
  },
  unit: {
    path: "/tenant/v1/units?pageSize=200",
    pick: (d) => (d as { units?: unknown[] })?.units ?? [],
    toOption: (u, locale) => ({
      id: str(u.id) ?? "",
      label: pickLabel(map(u.name), locale) || str(u.code) || str(u.id) || "",
      hint: str(u.code),
    }),
  },
  role: {
    path: "/authorization/v1/roles?pageSize=200",
    pick: (d) => (d as { roles?: unknown[] })?.roles ?? [],
    toOption: (r, locale) => ({
      id: str(r.id) ?? "",
      label: pickLabel(map(r.name), locale) || str(r.code) || str(r.id) || "",
      hint: str(r.code),
    }),
  },
  orderType: {
    path: "/order/v1/order-types",
    pick: (d) => (Array.isArray(d) ? d : ((d as { orderTypes?: unknown[] })?.orderTypes ?? [])),
    toOption: (t, locale) => ({
      id: str(t.id) ?? "",
      label: pickLabel(map(t.name), locale) || str(t.code) || str(t.id) || "",
      hint: str(t.code),
    }),
  },
};

export function EntitySelect({
  kind,
  name,
  defaultValue = "",
  required = false,
  placeholder = "Search…",
  allowEmpty = false,
  onChange,
}: {
  kind: EntityKind;
  name?: string;
  defaultValue?: string;
  required?: boolean;
  placeholder?: string;
  allowEmpty?: boolean;
  onChange?: (id: string) => void;
}) {
  const { locale } = useLocale();
  const cfg = REGISTRY[kind];
  const [items, setItems] = useState<Option[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadErr, setLoadErr] = useState(false);
  const [selected, setSelected] = useState<string>(defaultValue);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    fetch(`/api/oikumenea${cfg.path}`)
      .then((r) => (r.ok ? r.json() : Promise.reject(r)))
      .then((d) => {
        if (!alive) return;
        setItems(cfg.pick(d).map((it) => cfg.toOption(it as Record<string, unknown>, locale)));
        setLoading(false);
      })
      .catch(() => {
        if (!alive) return;
        setLoadErr(true);
        setLoading(false);
      });
    return () => {
      alive = false;
    };
    // refetch when locale changes so labels re-localize (cheap: ≤200 rows)
  }, [cfg, locale]);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) {
        setOpen(false);
        if (!selected) setQuery("");
      }
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [selected]);

  const selectedOption = items.find((o) => o.id === selected);
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const base = q
      ? items.filter(
          (o) => o.label.toLowerCase().includes(q) || (o.hint ?? "").toLowerCase().includes(q),
        )
      : items;
    return base.slice(0, 50);
  }, [items, query]);

  function choose(o: Option) {
    setSelected(o.id);
    setQuery("");
    setOpen(false);
    onChange?.(o.id);
  }
  function clear() {
    setSelected("");
    setQuery("");
    onChange?.("");
  }

  const selectedText = selectedOption
    ? `${selectedOption.label}${selectedOption.hint ? ` (${selectedOption.hint})` : ""}`
    : selected; // fall back to the raw RID if it isn't in the loaded page
  const display = open ? query : selected ? selectedText : query;

  return (
    <div className="relative" ref={boxRef}>
      {name ? <input type="hidden" name={name} value={selected} /> : null}
      <input
        className="input"
        placeholder={loading ? "Loading…" : loadErr ? "(failed to load list)" : placeholder}
        value={display}
        required={required}
        autoComplete="off"
        onFocus={() => setOpen(true)}
        onChange={(e) => {
          setQuery(e.target.value);
          setOpen(true);
          setActive(0);
          if (selected) {
            setSelected("");
            onChange?.("");
          }
        }}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setOpen(true);
            setActive((a) => Math.min(a + 1, filtered.length - 1));
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            setActive((a) => Math.max(a - 1, 0));
          } else if (e.key === "Enter") {
            if (open && filtered[active]) {
              e.preventDefault();
              choose(filtered[active]);
            }
          } else if (e.key === "Escape") {
            setOpen(false);
            if (!selected) setQuery("");
          }
        }}
      />
      {selected && allowEmpty ? (
        <button
          type="button"
          onClick={clear}
          aria-label="Clear"
          className="absolute right-2 top-1.5 text-xs text-slate-400 hover:text-slate-600"
        >
          ✕
        </button>
      ) : null}
      {open && !loadErr ? (
        <div className="absolute z-20 mt-1 max-h-64 w-full overflow-auto rounded-md border border-slate-200 bg-white py-1 shadow-lg">
          {filtered.length === 0 ? (
            <div className="px-3 py-2 text-sm text-slate-400">
              {loading ? "Loading…" : "No matches"}
            </div>
          ) : (
            filtered.map((o, i) => (
              <button
                type="button"
                key={o.id}
                onMouseEnter={() => setActive(i)}
                onClick={() => choose(o)}
                className={`flex w-full items-center justify-between gap-2 px-3 py-1.5 text-left text-sm ${
                  i === active ? "bg-indigo-50" : "hover:bg-slate-50"
                }`}
              >
                <span className="truncate text-slate-800">{o.label}</span>
                {o.hint ? (
                  <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">{o.hint}</span>
                ) : null}
              </button>
            ))
          )}
        </div>
      ) : null}
    </div>
  );
}
