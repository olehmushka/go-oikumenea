"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { EXPLORABLE_TYPES } from "@/lib/ontology/registry";
import { useT } from "@/lib/locale";
import { tg } from "@/lib/messages";

// Tools are surfaces that aren't object-table shaped (PDP check, tree editors, the log). Label/hint
// are message-catalog keys (translated via useT) so the nav follows the UI locale.
//
// `requires` names a permission code the visitor must hold for the entry to be OFFERED. It is a UX
// affordance only — the server decides regardless (see lib/api/can.ts). This component is a client
// component and cannot ask the PDP itself, so the dashboard layout resolves the codes server-side and
// passes the outcome in via `grants`. An entry with no `requires` is always shown, which is the
// console's historical behaviour for everything else.
const TOOLS: { href: string; key: string; requires?: string }[] = [
  { href: "/ontology", key: "nav.ontology" },
  { href: "/authorize", key: "nav.authorize" },
  { href: "/roles", key: "nav.roles" },
  {
    href: "/service-principals",
    key: "nav.servicePrincipals",
    requires: "service-principal.read",
  },
  { href: "/organizations", key: "nav.organizations" },
  { href: "/domains", key: "nav.domains" },
  { href: "/graphs", key: "nav.graphs" },
  { href: "/memberships", key: "nav.memberships" },
  { href: "/orders", key: "nav.orders" },
  { href: "/documents", key: "nav.documents" },
  { href: "/ranks", key: "nav.ranks" },
  { href: "/locations", key: "nav.locations" },
  { href: "/education", key: "nav.education" },
  { href: "/companies", key: "nav.companies" },
  { href: "/vehicles", key: "nav.vehicles" },
  { href: "/finance", key: "nav.finance" },
  { href: "/external-orgs", key: "nav.externalOrgs" },
  { href: "/religion", key: "nav.religion" },
  { href: "/localization", key: "nav.localization" },
  { href: "/legal-basis", key: "nav.legalBasis" },
  { href: "/imports", key: "nav.imports" },
  {
    href: "/connectors",
    key: "nav.connectors",
    requires: "connector.read",
  },
  { href: "/audit", key: "nav.audit" },
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
 * `grants` maps a permission code → whether the visitor holds it, resolved server-side by the
 * dashboard layout. Absent (or an unlisted code) means "not held", so a gated entry stays hidden if
 * the check could not be made — failing closed on a *display* decision.
 */
export function Nav({ grants = {} }: { grants?: Record<string, boolean> }) {
  const pathname = usePathname();
  const tr = useT();
  const offered = TOOLS.filter((item) => !item.requires || grants[item.requires] === true);
  return (
    <nav className="flex flex-col gap-0.5 p-3">
      <Item href="/" label={tr("nav.overview")} hint={tr("nav.overview.hint")} active={pathname === "/"} />

      <div className="mt-4 mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
        {tr("nav.section.explore")}
      </div>
      {EXPLORABLE_TYPES.map((type) => {
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
