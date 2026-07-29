"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Command } from "cmdk";
import { EXPLORABLE_TYPES, typeDef, readCodeFor } from "@/lib/ontology/registry";
import { type Capabilities, holds } from "@/lib/ontology/caps";
import { parseRid, ridType } from "@/lib/ontology/rid";
import { api } from "@/lib/api/client";
import { pushRecent } from "@/lib/ontology/recents";
import { useLocale } from "@/lib/locale";
import { setActiveLocale } from "@/lib/i18n";
import { tg } from "@/lib/messages";
import type { search as searchapi } from "oikumenea-client";

// Object search is SERVER-SIDE (D-UnifiedSearch, review-2026-09 R-26): one call to
// SearchService.searchObjects fans in every registered type's trigram index, permission-gated and
// visibility-trimmed per type — replacing the old client-side first-page-per-type fan-out cache,
// which silently missed anything beyond each type's first 100 rows.
const PER_TYPE_LIMIT = 5;
const PAGE_SIZE = 40;

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

// The badge text for a hit: the registry label when the type is known to the console, else the
// server's objectType token (a type can be searchable before it has a web registry entry).
function hitKind(hit: searchapi.ISearchHit): string {
  const token = ridType(hit.rid) ?? hit.objectType;
  return typeDef(token)?.label ?? hit.objectType;
}

export function CommandPalette({ caps }: { caps: Capabilities }) {
  const router = useRouter();
  // Subscribe to the UI locale so navigation/action labels re-render on switch (object hit labels
  // come from the server in the default locale).
  const { locale } = useLocale();
  setActiveLocale(locale);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<searchapi.ISearchHit[]>([]);
  const [loading, setLoading] = useState(false);
  const debounce = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const requestSeq = useRef(0);

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

  // Query the unified search endpoint as the user types (debounced; stale responses dropped).
  useEffect(() => {
    if (debounce.current) clearTimeout(debounce.current);
    const q = query.trim();
    if (q.length < 2) {
      setHits([]);
      setLoading(false);
      return;
    }
    const seq = ++requestSeq.current;
    setLoading(true);
    debounce.current = setTimeout(async () => {
      try {
        const page = await api.search.searchObjects(q, undefined, PER_TYPE_LIMIT, PAGE_SIZE);
        if (seq !== requestSeq.current) return;
        setHits(page.hits);
      } catch {
        if (seq !== requestSeq.current) return;
        setHits([]);
      } finally {
        if (seq === requestSeq.current) setLoading(false);
      }
    }, 150);
  }, [query]);

  const go = useCallback(
    (href: string) => {
      setOpen(false);
      setQuery("");
      router.push(href);
    },
    [router],
  );

  const openHit = useCallback(
    (hit: searchapi.ISearchHit) => {
      pushRecent({ id: hit.rid, type: ridType(hit.rid) ?? hit.objectType, label: hit.label });
      go(`/o/${encodeURIComponent(hit.rid)}`);
    },
    [go],
  );

  // Exact RID paste → jump straight there (the RID is self-describing).
  const ridHit = parseRid(query.trim());
  const ridKnown = ridHit && typeDef(ridHit.type);

  const q = query.trim().toLowerCase();
  // Only offer navigation the caller can actually use (D-SelfCapabilities). `requires: undefined`
  // means ungated (Overview, Ontology browser). Cosmetic only — the server re-decides on navigation.
  const navMatches = [
    ...EXPLORABLE_TYPES.map((t) => ({
      label: tg(t.labelPlural),
      href: `/explore/${t.type}`,
      requires: readCodeFor(t),
    })),
    { label: tg("Overview"), href: "/", requires: undefined },
    { label: tg("Ontology"), href: "/ontology", requires: undefined },
    { label: tg("Authorize"), href: "/authorize", requires: "assignment.read" },
    // The tool pages, named apart from the explore entries above: /explore/link__member_of and
    // /explore/order are the global faceted lists; these are the lookup and the issue desk.
    { label: tg("Membership lookup"), href: "/memberships", requires: "membership.read" },
    { label: tg("Order desk"), href: "/orders", requires: "order.read" },
    { label: tg("Ranks"), href: "/ranks", requires: "rank.scheme.read" },
    { label: tg("Localization"), href: "/localization", requires: "locale.read" },
    { label: tg("Audit"), href: "/audit", requires: "audit.read" },
  ].filter((n) => holds(caps, n.requires) && (!q || n.label.toLowerCase().includes(q)));

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
        {loading ? <div className="cmdk-status">{tg("Searching…")}</div> : null}

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
            {hits.map((hit, i) => (
              <Command.Item
                key={`${hit.rid}-${i}`}
                value={`obj-${hit.rid}-${i}`}
                onSelect={() => openHit(hit)}
              >
                <span className="cmdk-kind">{tg(hitKind(hit))}</span>
                <span className="truncate">{hit.label}</span>
                {hit.snippet ? <span className="cmdk-hint">{hit.snippet}</span> : null}
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
