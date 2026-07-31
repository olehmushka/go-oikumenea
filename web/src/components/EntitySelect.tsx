"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import { UnitSelect } from "@/components/UnitSelect";

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
export type EntityKind =
  | "person"
  | "unit"
  | "role"
  | "orderType"
  | "documentType"
  | "domain"
  | "rank"
  | "institution"
  | "publication"
  | "scholarship"
  | "externalOrgKind"
  | "taxon"
  | "taxonRank"
  | "classification";

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
  documentType: {
    path: "/document/v1/document-types",
    pick: (d) => (Array.isArray(d) ? d : ((d as { documentTypes?: unknown[] })?.documentTypes ?? [])),
    toOption: (t, locale) => ({
      id: str(t.id) ?? "",
      label: pickLabel(map(t.name), locale) || str(t.code) || str(t.id) || "",
      hint: str(t.code),
    }),
  },
  domain: {
    path: "/tenant/v1/domains",
    pick: (d) => (d as { domains?: unknown[] })?.domains ?? [],
    toOption: (d, locale) => ({
      id: str(d.id) ?? "",
      label: pickLabel(map(d.name), locale) || str(d.code) || str(d.id) || "",
      hint: str(d.code),
    }),
  },
  rank: {
    // The scheme is returned whole (systems → categories → types → ranks) in SENIORITY order; flatten
    // to the leaf ranks, which is what person.rankId stores. Order is preserved deliberately — an
    // ordered scheme re-sorted alphabetically loses the only ordering that means anything.
    path: "/rank/v1/rank-scheme",
    pick: (d) => {
      const out: unknown[] = [];
      // Types form a TREE (a type may nest under another via `children`), and ranks attach to LEAF
      // types — so this recurses rather than walking a fixed three levels.
      const walkTypes = (types: Record<string, unknown>[]) => {
        for (const ty of types) {
          out.push(...((ty.ranks ?? []) as unknown[]));
          walkTypes((ty.children ?? []) as Record<string, unknown>[]);
        }
      };
      for (const sys of ((d as { systems?: Record<string, unknown>[] })?.systems ?? [])) {
        for (const cat of ((sys.categories ?? []) as Record<string, unknown>[])) {
          walkTypes((cat.types ?? []) as Record<string, unknown>[]);
        }
      }
      return out;
    },
    toOption: (r, locale) => ({
      id: str(r.id) ?? "",
      label: pickLabel(map(r.name), locale) || str(r.code) || str(r.id) || "",
      hint: str(r.abbreviation) || str(r.code),
    }),
  },
  // Education (M20/M21) globally-scoped pickers. Institution-scoped child lists (units, grants,
  // research-groups, …) are loaded into plain <select>s by PersonEducation once an institution is chosen.
  institution: {
    path: "/education/v1/institutions?pageSize=200",
    pick: (d) => (d as { institutions?: unknown[] })?.institutions ?? [],
    toOption: (i, locale) => ({
      id: str(i.id) ?? "",
      label: pickLabel(map(i.name), locale) || str(i.code) || str(i.id) || "",
      hint: str(i.code),
    }),
  },
  publication: {
    // Keyset-paginated since review R-21 (was an unbounded list) — request a full page so the
    // client-side filter still sees the whole catalog at admin/dev scale.
    path: "/education/v1/publications?pageSize=200",
    pick: (d) => (d as { publications?: unknown[] })?.publications ?? [],
    toOption: (p) => ({
      id: str(p.id) ?? "",
      label: str(p.title) || str(p.code) || str(p.id) || "",
      hint: str(p.code),
    }),
  },
  scholarship: {
    path: "/education/v1/scholarships?pageSize=200",
    pick: (d) => (d as { scholarships?: unknown[] })?.scholarships ?? [],
    toOption: (s) => ({
      id: str(s.id) ?? "",
      label: str(s.name) || str(s.code) || str(s.id) || "",
      hint: str(s.code),
    }),
  },
  // M58 ticket 2 — the ref-facet pickers for the external-organization and taxonomy dashboards.
  // Each is a small closed catalog, so a single page is the whole set rather than a truncation.
  externalOrgKind: {
    path: "/external-orgs/v1/external-org-kinds",
    pick: (d) => (d as { kinds?: unknown[] })?.kinds ?? [],
    toOption: (k, locale) => ({
      id: str(k.id) ?? "",
      label: pickLabel(map(k.name), locale) || str(k.code) || str(k.id) || "",
      hint: str(k.code),
    }),
  },
  taxon: {
    // The taxonomy is hundreds of nodes, not thousands, so one page is the whole tree. It serves BOTH
    // the `religionId` filter (roots) and the `subtree` filter (any ancestor): one table, one picker.
    // rankCode is the hint, because "Baptists" means little without "denomination" beside it.
    path: "/religion/v1/taxa?pageSize=500",
    pick: (d) => (d as { taxa?: unknown[] })?.taxa ?? [],
    toOption: (t, locale) => ({
      id: str(t.id) ?? "",
      label: pickLabel(map(t.name), locale) || str(t.code) || str(t.id) || "",
      hint: str(t.rankCode) || str(t.code),
    }),
  },
  taxonRank: {
    path: "/religion/v1/taxon-ranks",
    pick: (d) => (d as { taxonRanks?: unknown[] })?.taxonRanks ?? [],
    toOption: (r, locale) => ({
      id: str(r.id) ?? "",
      label: pickLabel(map(r.name), locale) || str(r.code) || str(r.id) || "",
      hint: str(r.code),
    }),
  },
  classification: {
    path: "/religion/v1/classifications",
    pick: (d) => (d as { classifications?: unknown[] })?.classifications ?? [],
    toOption: (c, locale) => ({
      id: str(c.id) ?? "",
      label: pickLabel(map(c.name), locale) || str(c.code) || str(c.id) || "",
      hint: str(c.code),
    }),
  },
};

type EntitySelectProps = {
  kind: EntityKind;
  name?: string;
  defaultValue?: string;
  /** the human label for defaultValue; only kind="unit" needs it (the flat kinds resolve their own
   *  labels from the page they fetch), but it is accepted uniformly so callers need not branch. */
  defaultLabel?: string;
  required?: boolean;
  placeholder?: string;
  allowEmpty?: boolean;
  onChange?: (id: string, label?: string) => void;
};

/**
 * Units are org-scoped: the API rejects a flat `/tenant/v1/units` listing (D-TenantOrganizations,
 * M40), so unit pickers delegate to the org→units cascade in UnitSelect. Every other kind is a flat
 * global list and uses the searchable dropdown below. Branching in a wrapper (not an early return)
 * keeps each component's hooks unconditional.
 */
export function EntitySelect(props: EntitySelectProps) {
  if (props.kind === "unit") {
    return (
      <UnitSelect
        name={props.name}
        defaultValue={props.defaultValue}
        defaultLabel={props.defaultLabel}
        required={props.required}
        allowEmpty={props.allowEmpty}
        placeholder={props.placeholder}
        onChange={props.onChange}
      />
    );
  }
  return <FlatEntitySelect {...props} />;
}

function FlatEntitySelect({
  kind,
  name,
  defaultValue = "",
  required = false,
  placeholder = "Search…",
  allowEmpty = false,
  onChange,
}: EntitySelectProps) {
  const { locale } = useLocale();
  const tr = useTg();
  const cfg = REGISTRY[kind];
  const [items, setItems] = useState<Option[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadErr, setLoadErr] = useState(false);
  const [selected, setSelected] = useState<string>(defaultValue);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const boxRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  // Fixed-viewport coordinates for the portaled dropdown (so an overflow-hidden ancestor — e.g. a
  // table card — can't clip it).
  const [menuPos, setMenuPos] = useState<{ left: number; top: number; width: number } | null>(null);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    api
      .request(`GET`, cfg.path)
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
      const t = e.target as Node;
      const inBox = boxRef.current?.contains(t);
      const inPanel = panelRef.current?.contains(t);
      if (!inBox && !inPanel) {
        setOpen(false);
        if (!selected) setQuery("");
      }
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [selected]);

  // Track the input's viewport rect while the menu is open so the portaled dropdown stays anchored
  // (also follows scroll/resize).
  useEffect(() => {
    if (!open) return;
    const update = () => {
      const r = inputRef.current?.getBoundingClientRect();
      if (r) setMenuPos({ left: r.left, top: r.bottom + 4, width: r.width });
    };
    update();
    window.addEventListener("scroll", update, true);
    window.addEventListener("resize", update);
    return () => {
      window.removeEventListener("scroll", update, true);
      window.removeEventListener("resize", update);
    };
  }, [open]);

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
    onChange?.(o.id, o.label);
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
        ref={inputRef}
        className="input"
        placeholder={loading ? tr("Loading…") : loadErr ? tr("(failed to load list)") : tr(placeholder)}
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
      {open && !loadErr && menuPos
        ? createPortal(
            <div
              ref={panelRef}
              style={{ position: "fixed", left: menuPos.left, top: menuPos.top, width: menuPos.width }}
              className="z-50 max-h-64 overflow-auto rounded-md border border-slate-200 bg-white py-1 shadow-lg"
            >
              {filtered.length === 0 ? (
                <div className="px-3 py-2 text-sm text-slate-400">
                  {loading ? tr("Loading…") : tr("No matches")}
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
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}
