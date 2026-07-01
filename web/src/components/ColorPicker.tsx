"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";

/**
 * Per-domain color picker (D-Color). Lists the palette for a single `domain` (eye | hair | vehicle)
 * from the platform color catalog, renders a hex swatch + localized name per option, and — the key
 * affordance — lets an operator CREATE a new color in place: typing a name with no match offers a
 * "Create '…'" button that upserts `{domain, code, name, hex?}` then selects it. The opaque color RID
 * is submitted (controlled `onChange`, or a hidden <input name=…> for FormData forms), mirroring the
 * LanguagePicker.
 */
type Color = {
  id: string;
  domain: string;
  code: string;
  name?: Record<string, string>;
  hex?: string | null;
};

// slugify turns a free-text name into a stable, locale-agnostic code (D-Code): "Dark Brown" -> "dark-brown".
function slugify(s: string): string {
  return s
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function Swatch({ hex }: { hex?: string | null }) {
  return (
    <span
      className="inline-block h-3.5 w-3.5 shrink-0 rounded-sm border border-slate-300"
      style={{ backgroundColor: hex ?? "transparent" }}
      aria-hidden
    />
  );
}

export function ColorPicker({
  domain,
  name,
  value,
  onChange,
  placeholder = "Search or create a color…",
}: {
  domain: "eye" | "hair" | "vehicle";
  name?: string;
  value?: string;
  onChange?: (id: string) => void;
  placeholder?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [colors, setColors] = useState<Color[] | null>(null);
  const [selected, setSelected] = useState<{ id: string; label: string; hex?: string | null } | null>(null);
  const [creating, setCreating] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  function labelFor(c: Color) {
    return pickLabel(c.name, locale) || c.code || c.id;
  }

  // Load the palette once on first focus.
  function load() {
    if (colors) return;
    api.platformCatalog
      .listColors(domain)
      .then((r) => setColors((r?.colors ?? []) as unknown as Color[]))
      .catch(() => setColors([]));
  }

  // Resolve an externally-supplied value to its label once the palette is loaded.
  useEffect(() => {
    if (!value) {
      setSelected(null);
      return;
    }
    if (selected?.id === value) return;
    api.platformCatalog
      .listColors(domain)
      .then((r) => {
        const list = (r?.colors ?? []) as unknown as Color[];
        setColors(list);
        const c = list.find((x) => x.id === value);
        if (c) setSelected({ id: c.id, label: labelFor(c), hex: c.hex });
      })
      .catch(() => undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, domain]);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  const q = query.trim().toLowerCase();
  const matches = useMemo(() => {
    const list = colors ?? [];
    if (!q) return list;
    return list.filter((c) => labelFor(c).toLowerCase().includes(q) || c.code.toLowerCase().includes(q));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [colors, q, locale]);

  // An exact match (by code) means "create" would just re-upsert; only offer create when none matches.
  const canCreate = q.length > 0 && !(colors ?? []).some((c) => c.code === slugify(query) || labelFor(c).toLowerCase() === q);

  function choose(c: Color) {
    setSelected({ id: c.id, label: labelFor(c), hex: c.hex });
    setOpen(false);
    setQuery("");
    onChange?.(c.id);
  }

  function clear() {
    setSelected(null);
    setQuery("");
    onChange?.("");
  }

  function createColor() {
    const display = query.trim();
    if (!display || creating) return;
    setCreating(true);
    api.platformCatalog
      .upsertColor({ domain, code: slugify(display), name: display, hex: null, sortOrder: null })
      .then((c) => {
        setColors((list) => [...(list ?? []), c as unknown as Color]);
        choose(c as unknown as Color);
      })
      .catch(() => undefined)
      .finally(() => setCreating(false));
  }

  return (
    <div className="relative" ref={boxRef}>
      {name ? <input type="hidden" name={name} value={selected?.id ?? ""} /> : null}
      {selected ? (
        <div className="flex items-center gap-2">
          <span className="input flex flex-1 items-center gap-2 truncate bg-slate-50">
            <Swatch hex={selected.hex} />
            <span className="truncate">{selected.label}</span>
          </span>
          <button type="button" className="text-xs text-red-600 hover:underline" onClick={clear}>
            {tr("clear")}
          </button>
        </div>
      ) : (
        <input
          className="input"
          placeholder={tr(placeholder)}
          value={query}
          autoComplete="off"
          onFocus={() => {
            load();
            setOpen(true);
          }}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
          }}
        />
      )}
      {open && !selected ? (
        <div className="absolute z-10 mt-1 max-h-72 w-full overflow-auto rounded-md border border-slate-200 bg-white p-1 shadow-lg">
          {colors === null ? (
            <div className="px-2 py-1.5 text-sm text-slate-400">{tr("Loading…")}</div>
          ) : (
            <>
              {matches.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className="flex w-full items-center gap-2 truncate rounded px-2 py-1 text-left text-sm hover:bg-indigo-50"
                  onClick={() => choose(c)}
                >
                  <Swatch hex={c.hex} />
                  <span className="flex-1 truncate">{labelFor(c)}</span>
                  <span className="ml-2 shrink-0 font-mono text-xs text-slate-400">{c.code}</span>
                </button>
              ))}
              {matches.length === 0 && !canCreate ? (
                <div className="px-2 py-1.5 text-sm text-slate-400">{tr("No colors")}</div>
              ) : null}
              {canCreate ? (
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-sm text-indigo-700 hover:bg-indigo-50"
                  onClick={createColor}
                  disabled={creating}
                >
                  {creating ? tr("Creating…") : `${tr("Create")} “${query.trim()}”`}
                </button>
              ) : null}
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}
