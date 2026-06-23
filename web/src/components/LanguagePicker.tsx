"use client";

import { useEffect, useRef, useState } from "react";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";

/**
 * Searchable Glottolog language picker (D-Languages, M18). Unlike EntitySelect (one page +
 * client-side filter), the languoid catalog is ~27k rows, so this hits the LanguageService server
 * `query` filter as the user types (debounced) and submits the opaque languoid RID. Restricted to
 * level='language' — the only level the SPEAKS / unit-language links accept.
 *
 * Modes: `name` renders a hidden <input name=…> with the RID (FormData forms); `onChange` is the
 * controlled callback.
 */
type Languoid = { id: string; code?: string; name?: Record<string, string>; iso6393?: string };

export function LanguagePicker({
  name,
  onChange,
  placeholder = "Search a language…",
}: {
  name?: string;
  onChange?: (id: string) => void;
  placeholder?: string;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Languoid[]>([]);
  const [selected, setSelected] = useState<{ id: string; label: string } | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, []);

  useEffect(() => {
    const q = query.trim();
    if (selected || q.length < 2) {
      setResults([]);
      return;
    }
    let alive = true;
    setLoading(true);
    const t = setTimeout(() => {
      fetch(`/api/oikumenea/language/v1/languages?level=language&query=${encodeURIComponent(q)}&limit=20`)
        .then((r) => (r.ok ? r.json() : Promise.reject(r)))
        .then((d) => {
          if (!alive) return;
          setResults((d?.languoids ?? []) as Languoid[]);
          setLoading(false);
        })
        .catch(() => alive && setLoading(false));
    }, 200);
    return () => {
      alive = false;
      clearTimeout(t);
    };
  }, [query, selected]);

  function choose(l: Languoid) {
    const label = `${pickLabel(l.name, locale) || l.code || l.id}${l.iso6393 ? ` (${l.iso6393})` : ""}`;
    setSelected({ id: l.id, label });
    setOpen(false);
    setQuery("");
    onChange?.(l.id);
  }
  function clear() {
    setSelected(null);
    setQuery("");
    onChange?.("");
  }

  return (
    <div className="relative" ref={boxRef}>
      {name ? <input type="hidden" name={name} value={selected?.id ?? ""} /> : null}
      {selected ? (
        <div className="flex items-center gap-2">
          <span className="input flex-1 truncate bg-slate-50">{selected.label}</span>
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
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
          }}
        />
      )}
      {open && !selected && (query.trim().length >= 2 || results.length > 0) ? (
        <div className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md border border-slate-200 bg-white shadow-lg">
          {loading ? (
            <div className="px-3 py-2 text-sm text-slate-400">{tr("Searching…")}</div>
          ) : results.length === 0 ? (
            <div className="px-3 py-2 text-sm text-slate-400">{tr("No matches")}</div>
          ) : (
            results.map((l) => (
              <button
                key={l.id}
                type="button"
                className="flex w-full items-center justify-between px-3 py-1.5 text-left text-sm hover:bg-indigo-50"
                onClick={() => choose(l)}
              >
                <span>{pickLabel(l.name, locale) || l.code}</span>
                <span className="font-mono text-xs text-slate-400">{l.iso6393 || l.code}</span>
              </button>
            ))
          )}
        </div>
      ) : null}
    </div>
  );
}
