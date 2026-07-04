"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { EXPLORABLE_TYPES } from "@/lib/ontology/registry";
import { useT } from "@/lib/locale";
import { tg } from "@/lib/messages";

// Tools are surfaces that aren't object-table shaped (PDP check, tree editors, the log). Label/hint
// are message-catalog keys (translated via useT) so the nav follows the UI locale.
const TOOLS: { href: string; key: string }[] = [
  { href: "/ontology", key: "nav.ontology" },
  { href: "/authorize", key: "nav.authorize" },
  { href: "/roles", key: "nav.roles" },
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
  { href: "/external-orgs", key: "nav.externalOrgs" },
  { href: "/religion", key: "nav.religion" },
  { href: "/localization", key: "nav.localization" },
  { href: "/legal-basis", key: "nav.legalBasis" },
  { href: "/imports", key: "nav.imports" },
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

export function Nav() {
  const pathname = usePathname();
  const tr = useT();
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
      {TOOLS.map((item) => (
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
