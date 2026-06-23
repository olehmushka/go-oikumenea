"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Command } from "cmdk";
import {
  EXPLORABLE_TYPES,
  OBJECT_TYPES,
  rowSearchText,
  typeDef,
  type ObjectTypeDef,
  type Row,
} from "@/lib/ontology/registry";
import { parseRid } from "@/lib/ontology/rid";
import { api } from "@/lib/api/client";
import { pushRecent } from "@/lib/ontology/recents";
import { useLocale } from "@/lib/locale";
import { setActiveLocale } from "@/lib/i18n";
import { tg } from "@/lib/messages";

// ── fan-out object cache (no server-side search exists; we fetch one page per type and filter in the
// browser). Module-level so it survives palette re-opens within a session; short TTL keeps it fresh. ──
type Cache = { at: number; byType: Record<string, Row[]> };
let CACHE: Cache | null = null;
let INFLIGHT: Promise<Cache> | null = null;
const TTL_MS = 60_000;

async function loadIndex(): Promise<Cache> {
  if (CACHE && Date.now() - CACHE.at < TTL_MS) return CACHE;
  if (INFLIGHT) return INFLIGHT;
  INFLIGHT = (async () => {
    const byType: Record<string, Row[]> = {};
    await Promise.all(
      EXPLORABLE_TYPES.map(async (def) => {
        try {
          const search = def.list!.search ?? "?pageSize=100";
          const res = await api.request(`GET`, `${def.list!.path}${search}`);
          byType[def.type] = def.list!.parse(res).rows;
        } catch {
          byType[def.type] = [];
        }
      }),
    );
    CACHE = { at: Date.now(), byType };
    INFLIGHT = null;
    return CACHE;
  })();
  return INFLIGHT;
}

// ── static quick actions ──
interface QuickAction {
  label: string;
  hint?: string;
  run: (ctx: { router: ReturnType<typeof useRouter> }) => void | Promise<void>;
}
const QUICK_ACTIONS: QuickAction[] = [
  { label: "New person", hint: "create", run: ({ router }) => router.push("/persons/new") },
  { label: "New unit", hint: "create", run: ({ router }) => router.push("/units/new") },
  { label: "Authorize check", hint: "PDP", run: ({ router }) => router.push("/authorize") },
  { label: "Ontology browser", hint: "types", run: ({ router }) => router.push("/ontology") },
  {
    label: "Rebuild unit closure",
    hint: "tenant",
    run: async ({ router }) => {
      await api.tenant.rebuildClosure();
      router.refresh();
    },
  },
];

interface ObjectHit {
  def: ObjectTypeDef;
  row: Row;
}

export function CommandPalette() {
  const router = useRouter();
  // Subscribe to the UI locale so result titles/subtitles re-render in the chosen locale on switch
  // (the cached rows keep their full `locale → text` maps; only the picked label changes).
  const { locale } = useLocale();
  setActiveLocale(locale);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<ObjectHit[]>([]);
  const [loading, setLoading] = useState(false);
  const debounce = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  // ⌘K / Ctrl-K toggles the palette anywhere.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    const onOpen = () => setOpen(true);
    document.addEventListener("keydown", onKey);
    window.addEventListener("oik:open-palette", onOpen);
    return () => {
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("oik:open-palette", onOpen);
    };
  }, []);

  // Warm the fan-out index when the palette opens.
  useEffect(() => {
    if (!open) return;
    setLoading(true);
    loadIndex().finally(() => setLoading(false));
  }, [open]);

  // Filter the cached objects as the user types (debounced; client-side substring match).
  useEffect(() => {
    if (debounce.current) clearTimeout(debounce.current);
    const q = query.trim().toLowerCase();
    if (q.length < 2) {
      setHits([]);
      return;
    }
    debounce.current = setTimeout(async () => {
      const idx = await loadIndex();
      const out: ObjectHit[] = [];
      for (const def of EXPLORABLE_TYPES) {
        for (const row of idx.byType[def.type] ?? []) {
          if (rowSearchText(def, row).includes(q)) out.push({ def, row });
          if (out.length >= 40) break;
        }
      }
      setHits(out);
    }, 120);
  }, [query]);

  const go = useCallback(
    (href: string) => {
      setOpen(false);
      setQuery("");
      router.push(href);
    },
    [router],
  );

  const openObject = useCallback(
    (def: ObjectTypeDef, row: Row) => {
      pushRecent({ id: row.id, type: def.type, label: def.title(row) });
      go(`/o/${encodeURIComponent(row.id)}`);
    },
    [go],
  );

  // Exact RID paste → jump straight there (the RID is self-describing).
  const ridHit = parseRid(query.trim());
  const ridKnown = ridHit && typeDef(ridHit.type);

  const q = query.trim().toLowerCase();
  const navMatches = [
    ...EXPLORABLE_TYPES.map((t) => ({ label: tg(t.labelPlural), href: `/explore/${t.type}` })),
    { label: tg("Overview"), href: "/" },
    { label: tg("Ontology"), href: "/ontology" },
    { label: tg("Authorize"), href: "/authorize" },
    { label: tg("Memberships"), href: "/memberships" },
    { label: tg("Orders"), href: "/orders" },
    { label: tg("Ranks"), href: "/ranks" },
    { label: tg("Localization"), href: "/localization" },
    { label: tg("Audit"), href: "/audit" },
  ].filter((n) => !q || n.label.toLowerCase().includes(q));

  const actionMatches = QUICK_ACTIONS.filter((a) => !q || a.label.toLowerCase().includes(q));

  return (
    <Command.Dialog
      open={open}
      onOpenChange={setOpen}
      shouldFilter={false}
      label="Command palette"
      className="cmdk-root"
    >
      <Command.Input
        value={query}
        onValueChange={setQuery}
        placeholder={tg("Search objects, jump to a view, or paste a RID…")}
        autoFocus
      />
      <Command.List>
        {loading ? <div className="cmdk-status">{tg("Indexing…")}</div> : null}

        {ridKnown ? (
          <Command.Group heading={tg("Open")}>
            <Command.Item
              value={`open-${ridHit!.type}`}
              onSelect={() => go(`/o/${encodeURIComponent(query.trim())}`)}
            >
              <span className="cmdk-kind">{tg(typeDef(ridHit!.type)!.label)}</span>
              <span className="cmdk-mono">{ridHit!.uuid.slice(-12)}</span>
            </Command.Item>
          </Command.Group>
        ) : null}

        {navMatches.length > 0 ? (
          <Command.Group heading={tg("Navigate")}>
            {navMatches.map((n) => (
              <Command.Item key={n.href} value={`nav-${n.href}`} onSelect={() => go(n.href)}>
                {n.label}
                <span className="cmdk-hint">{n.href}</span>
              </Command.Item>
            ))}
          </Command.Group>
        ) : null}

        {actionMatches.length > 0 ? (
          <Command.Group heading={tg("Actions")}>
            {actionMatches.map((a) => (
              <Command.Item
                key={a.label}
                value={`act-${a.label}`}
                onSelect={async () => {
                  setOpen(false);
                  setQuery("");
                  await a.run({ router });
                }}
              >
                {tg(a.label)}
                {a.hint ? <span className="cmdk-hint">{tg(a.hint)}</span> : null}
              </Command.Item>
            ))}
          </Command.Group>
        ) : null}

        {hits.length > 0 ? (
          <Command.Group heading={tg("Objects")}>
            {hits.map(({ def, row }, i) => (
              <Command.Item
                key={`${row.id}-${i}`}
                value={`obj-${row.id}-${i}`}
                onSelect={() => openObject(def, row)}
              >
                <span className="cmdk-kind">{tg(def.label)}</span>
                <span className="truncate">{def.title(row)}</span>
                {def.subtitle?.(row) ? (
                  <span className="cmdk-hint">{def.subtitle(row)}</span>
                ) : null}
              </Command.Item>
            ))}
          </Command.Group>
        ) : null}

        <Command.Empty className="cmdk-status">
          {q.length < 2 ? tg("Type to search…") : tg("No matches.")}
        </Command.Empty>
      </Command.List>
    </Command.Dialog>
  );
}
