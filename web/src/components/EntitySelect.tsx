"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { api } from "@/lib/api/client";
import { useLocale, useTg } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import { UnitSelect } from "@/components/UnitSelect";

/**
 * Searchable entity picker (D-WebUI UX): replaces free-text RID inputs with a type-to-filter
 * dropdown that shows human labels (name/code) and submits the opaque RID.
 *
 * Two search modes, declared per kind by `searchParam`:
 *
 *  - **Server-side** (`searchParam` set — person, institution, taxon, brand). The typed query is
 *    debounced and sent to the list endpoint, which matches with a trigram index. REQUIRED for any
 *    directory that can outgrow one page: the picker loads `pageSize=200`, so with a client-side
 *    filter alone a 304-person directory left 104 people unselectable — invisible, because the
 *    dropdown looks like it is searching and simply reports "No matches".
 *  - **Client-side** (no `searchParam` — roles, catalogs, type registries). Bounded enumerations
 *    where one page genuinely is the whole set, and whose endpoints take no query param.
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
  | "classification"
  | "vehicleType"
  | "brand"
  | "color"
  | "accountType"
  | "cardNetwork"
  | "legalForm"
  | "industryClass"
  | "institutionKind"
  | "locationType"
  | "graph";

type Option = { id: string; label: string; hint?: string };

type KindConfig = {
  path: string; // includes any query string
  /**
   * Query-parameter name this kind's list endpoint accepts for server-side search. Set it ONLY when
   * the endpoint really supports it (Conjure `args.query`) — a param the server ignores would be
   * silently dropped, leaving the picker filtering one page while appearing to search everything.
   */
  searchParam?: string;
  pick: (data: unknown) => unknown[];
  toOption: (item: Record<string, unknown>, locale: string) => Option;
};

const str = (v: unknown): string | undefined => (typeof v === "string" ? v : undefined);
const map = (v: unknown) => (v && typeof v === "object" ? (v as Record<string, string>) : undefined);

const REGISTRY: Record<EntityKind, KindConfig> = {
  person: {
    // The directory is the one picker that reliably outgrows a page — hence server-side search
    // (D-PersonSearch: trigram-indexed over display name, code, given/surname AND name variants,
    // so a transliteration or alias matches too, which no client-side label filter could do).
    path: "/person/v1/persons?pageSize=200",
    searchParam: "query",
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
    searchParam: "query",
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
    searchParam: "query",
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
  // M58 ticket 3 — the ref-facet pickers for the vehicle, account and card dashboards. Each is a
  // small closed catalog, so one page is the whole set rather than a truncation. `model` is absent
  // deliberately: models are listable only per brand, so the filter bar renders it as a ScopedSelect.
  vehicleType: {
    path: "/vehicle/v1/vehicle-types",
    pick: (d) => (d as { types?: unknown[] })?.types ?? [],
    toOption: (t, locale) => ({
      id: str(t.id) ?? "",
      label: pickLabel(map(t.name), locale) || str(t.code) || str(t.id) || "",
      hint: str(t.code),
    }),
  },
  brand: {
    path: "/vehicle/v1/brands",
    searchParam: "query",
    pick: (d) => (d as { brands?: unknown[] })?.brands ?? [],
    toOption: (b, locale) => ({
      id: str(b.id) ?? "",
      label: pickLabel(map(b.name), locale) || str(b.code) || str(b.id) || "",
      hint: str(b.code),
    }),
  },
  // Scoped to the vehicle palette: platform_colors is per-domain (eye | hair | vehicle), and offering
  // eye colours as a vehicle filter would list values no vehicle can hold (D-Color).
  color: {
    path: "/platform/v1/colors?domain=vehicle",
    pick: (d) => (d as { colors?: unknown[] })?.colors ?? [],
    toOption: (c, locale) => ({
      id: str(c.id) ?? "",
      label: pickLabel(map(c.name), locale) || str(c.code) || str(c.id) || "",
      hint: str(c.code),
    }),
  },
  accountType: {
    path: "/finance/v1/account-types",
    pick: (d) => (d as { types?: unknown[] })?.types ?? [],
    toOption: (t, locale) => ({
      id: str(t.id) ?? "",
      label: pickLabel(map(t.name), locale) || str(t.code) || str(t.id) || "",
      hint: str(t.code),
    }),
  },
  // M58 ticket 5 — the ref-facet pickers for the company and institution dashboards. All three are
  // small closed catalogs served whole in one page, like the ticket-3 set above. `institution` is
  // already registered further up as an object picker; what is added here is its KIND catalog.
  legalForm: {
    path: "/company/v1/legal-forms",
    pick: (d) => (d as { legalForms?: unknown[] })?.legalForms ?? [],
    toOption: (f, locale) => ({
      id: str(f.id) ?? "",
      label: pickLabel(map(f.name), locale) || str(f.code) || str(f.id) || "",
      hint: str(f.abbreviation) || str(f.code),
    }),
  },
  industryClass: {
    path: "/company/v1/industry-classes",
    pick: (d) => (d as { industryClasses?: unknown[] })?.industryClasses ?? [],
    toOption: (c, locale) => ({
      id: str(c.id) ?? "",
      label: pickLabel(map(c.name), locale) || str(c.code) || str(c.id) || "",
      // The classification SYSTEM (NACE / ISIC / KVED) is the disambiguator here: two systems can
      // carry the same code for different activities.
      hint: [str(c.system), str(c.code)].filter(Boolean).join(" "),
    }),
  },
  // M58 ticket 6. Both are small instance-level catalogs with no server-side search param — a page of
  // 200 covers three place types and a handful of graphs many times over, so the client-side filter
  // EntitySelect falls back to is honest here in a way it was not for the person directory.
  locationType: {
    path: "/location/v1/location/types",
    pick: (d) => (d as { locationTypes?: unknown[] })?.locationTypes ?? [],
    toOption: (t, locale) => ({
      id: str(t.id) ?? "",
      label: pickLabel(map(t.name), locale) || str(t.code) || str(t.id) || "",
      hint: str(t.code),
    }),
  },
  // Without `org` this returns the instance-global graphs only; the assignment filter wants every
  // graph a grant could name, so it passes no org and accepts that an org-local graph is filterable
  // by clicking its bar rather than by picking it from this list.
  graph: {
    path: "/tenant/v1/graphs",
    pick: (d) => (d as { graphs?: unknown[] })?.graphs ?? [],
    toOption: (g, locale) => ({
      id: str(g.id) ?? "",
      label: pickLabel(map(g.name), locale) || str(g.code) || str(g.id) || "",
      hint: str(g.code),
    }),
  },
  institutionKind: {
    path: "/education/v1/institution-kinds",
    pick: (d) => (d as { institutionKinds?: unknown[] })?.institutionKinds ?? [],
    toOption: (k, locale) => ({
      id: str(k.id) ?? "",
      label: pickLabel(map(k.name), locale) || str(k.code) || str(k.id) || "",
      hint: str(k.code),
    }),
  },
  cardNetwork: {
    path: "/finance/v1/card-networks",
    pick: (d) => (d as { networks?: unknown[] })?.networks ?? [],
    toOption: (n, locale) => ({
      id: str(n.id) ?? "",
      label: pickLabel(map(n.name), locale) || str(n.code) || str(n.id) || "",
      hint: str(n.code),
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

  // Debounced copy of the typed query, used only by server-searched kinds. 200ms is short enough to
  // feel immediate and long enough that typing a name is one request, not one per keystroke.
  const [debouncedQuery, setDebouncedQuery] = useState("");
  useEffect(() => {
    if (!cfg.searchParam) return;
    const t = setTimeout(() => setDebouncedQuery(query.trim()), 200);
    return () => clearTimeout(t);
  }, [query, cfg.searchParam]);

  // The term actually sent to the server. Client-filtered kinds always send nothing.
  const serverQuery = cfg.searchParam ? debouncedQuery : "";

  useEffect(() => {
    let alive = true;
    setLoading(true);
    const path =
      serverQuery.length > 0
        ? `${cfg.path}${cfg.path.includes("?") ? "&" : "?"}${cfg.searchParam}=${encodeURIComponent(serverQuery)}`
        : cfg.path;
    api
      .request(`GET`, path)
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
    // refetch when locale changes so labels re-localize, and when the debounced server query changes
  }, [cfg, locale, serverQuery]);

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

  // The chosen option is remembered rather than looked up in `items`: choosing clears the query,
  // which refetches the unfiltered first page — and a person found by search is frequently NOT on it,
  // so a pure lookup would blank the field's label right after a successful pick.
  const [chosen, setChosen] = useState<Option | null>(null);
  const selectedOption = chosen?.id === selected ? chosen : items.find((o) => o.id === selected);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    // Server-searched kinds are NOT re-filtered here. The server matches more than a label — a person
    // matches on their code and on transliterated name variants (D-PersonSearch) — so re-applying a
    // substring test against the label would throw away correct hits the user cannot otherwise reach.
    if (cfg.searchParam) return items.slice(0, 50);
    const base = q
      ? items.filter(
          (o) => o.label.toLowerCase().includes(q) || (o.hint ?? "").toLowerCase().includes(q),
        )
      : items;
    return base.slice(0, 50);
  }, [items, query, cfg.searchParam]);

  function choose(o: Option) {
    setSelected(o.id);
    setChosen(o);
    setQuery("");
    setOpen(false);
    onChange?.(o.id, o.label);
  }
  function clear() {
    setSelected("");
    setChosen(null);
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
            setChosen(null);
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
