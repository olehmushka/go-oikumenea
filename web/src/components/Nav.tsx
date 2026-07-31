"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { EXPLORABLE_TYPES, readCodeFor } from "@/lib/ontology/registry";
import { type Capabilities, NO_CAPS, holds } from "@/lib/ontology/caps";
import { useT } from "@/lib/locale";
import { tg } from "@/lib/messages";

// Tools are surfaces that aren't object-table shaped (PDP check, tree editors, the log). Label/hint
// are message-catalog keys (translated via useT) so the nav follows the UI locale.
//
// `requires` names the `<module>.read` code the visitor must hold for the entry to be OFFERED. It is a
// UX affordance only — the server decides regardless (see lib/api/can.ts). This is a client component
// and cannot ask the API itself, so the dashboard layout fetches the caller's capabilities server-side
// (D-SelfCapabilities) and passes them in via `caps`; we filter locally with `holds()`. An entry with
// no `requires` is always shown (e.g. the ontology browser, which gates its own cards).
const TOOLS: { href: string; key: string; requires?: string }[] = [
  { href: "/ontology", key: "nav.ontology" },
  { href: "/authorize", key: "nav.authorize", requires: "assignment.read" },
  { href: "/roles", key: "nav.roles", requires: "role.read" },
  {
    href: "/service-principals",
    key: "nav.servicePrincipals",
    requires: "service-principal.read",
  },
  { href: "/organizations", key: "nav.organizations", requires: "organization.read" },
  { href: "/domains", key: "nav.domains", requires: "domain.read" },
  { href: "/graphs", key: "nav.graphs", requires: "graph.read" },
  { href: "/memberships", key: "nav.memberships", requires: "membership.read" },
  { href: "/orders", key: "nav.orders", requires: "order.read" },
  { href: "/documents", key: "nav.documents", requires: "document.read" },
  { href: "/ranks", key: "nav.ranks", requires: "rank.scheme.read" },
  { href: "/locations", key: "nav.locations", requires: "location.read" },
  { href: "/education", key: "nav.education", requires: "education.read" },
  { href: "/companies", key: "nav.companies", requires: "company.read" },
  { href: "/vehicles", key: "nav.vehicles", requires: "vehicle.read" },
  { href: "/finance", key: "nav.finance", requires: "finance.read" },
  { href: "/external-orgs", key: "nav.externalOrgs", requires: "externalorg.read" },
  { href: "/religion", key: "nav.religion", requires: "religion.read" },
  { href: "/localization", key: "nav.localization", requires: "locale.read" },
  { href: "/legal-basis", key: "nav.legalBasis", requires: "legal-basis.read" },
  { href: "/imports", key: "nav.imports" },
  {
    href: "/connectors",
    key: "nav.connectors",
    requires: "connector.read",
  },
  // No /audit here since M58 ticket 1: the ledger is an EXPLORE entry now (list + dashboard over its
  // nine filters), and the old path redirects there. A tools link beside it would be two doors into
  // one room, which is what "Membership lookup" vs "/explore/link__member_of" earned its rename for.
];

function Item({
  href,
  label,
  hint,
  active,
}: {
  href: string;
  label: string;
  hint?: string;
  active: boolean;
}) {
  return (
    <Link
      href={href}
      className={`flex flex-col rounded-md px-3 py-1.5 ${
        active ? "bg-indigo-50 text-indigo-700" : "text-slate-700 hover:bg-slate-100"
      }`}
    >
      <span className="text-sm font-medium">{label}</span>
      {hint ? <span className="text-xs text-slate-400">{hint}</span> : null}
    </Link>
  );
}

/**
 * `caps` is the caller's own effective permissions, fetched server-side by the dashboard layout
 * (D-SelfCapabilities). A gated entry (nav tool or explore type) is drawn only when `holds()` says so;
 * failure to fetch defaults to NO_CAPS, hiding gated entries — failing closed on a *display* decision.
 */
export function Nav({ caps = NO_CAPS }: { caps?: Capabilities }) {
  const pathname = usePathname();
  const tr = useT();
  const offered = TOOLS.filter((item) => holds(caps, item.requires));
  const explorable = EXPLORABLE_TYPES.filter((type) => holds(caps, readCodeFor(type)));
  return (
    <nav className="flex flex-col gap-0.5 p-3">
      <Item href="/" label={tr("nav.overview")} hint={tr("nav.overview.hint")} active={pathname === "/"} />

      <div className="mt-4 mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
        {tr("nav.section.explore")}
      </div>
      {explorable.map((type) => {
        const href = `/explore/${type.type}`;
        return (
          <Item
            key={type.type}
            href={href}
            label={tg(type.labelPlural)}
            active={pathname.startsWith(href)}
          />
        );
      })}

      <div className="mt-4 mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
        {tr("nav.section.tools")}
      </div>
      {offered.map((item) => (
        <Item
          key={item.href}
          href={item.href}
          label={tr(item.key)}
          hint={tr(`${item.key}.hint`)}
          active={pathname.startsWith(item.href)}
        />
      ))}
    </nav>
  );
}
