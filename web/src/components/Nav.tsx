"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { EXPLORABLE_TYPES } from "@/lib/ontology/registry";

// Tools are surfaces that aren't object-table shaped (PDP check, tree editors, the log).
const TOOLS: { href: string; label: string; hint: string }[] = [
  { href: "/ontology", label: "Ontology", hint: "type registry" },
  { href: "/authorize", label: "Authorize", hint: "PDP check" },
  { href: "/roles", label: "Roles & access", hint: "RBAC + assignments" },
  { href: "/graphs", label: "Graph admin", hint: "hierarchies + closure" },
  { href: "/memberships", label: "Memberships", hint: "person ↔ unit" },
  { href: "/orders", label: "Orders", hint: "наказ" },
  { href: "/documents", label: "Documents", hint: "papers & catalogs" },
  { href: "/ranks", label: "Ranks", hint: "rank scheme" },
  { href: "/localization", label: "Localization", hint: "locales" },
  { href: "/audit", label: "Audit", hint: "log" },
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
  return (
    <nav className="flex flex-col gap-0.5 p-3">
      <Item href="/" label="Overview" hint="whoami & recents" active={pathname === "/"} />

      <div className="mt-4 mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
        Explore
      </div>
      {EXPLORABLE_TYPES.map((t) => {
        const href = `/explore/${t.type}`;
        return (
          <Item
            key={t.type}
            href={href}
            label={t.labelPlural}
            active={pathname.startsWith(href)}
          />
        );
      })}

      <div className="mt-4 mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-slate-400">
        Tools
      </div>
      {TOOLS.map((t) => (
        <Item
          key={t.href}
          href={t.href}
          label={t.label}
          hint={t.hint}
          active={pathname.startsWith(t.href)}
        />
      ))}
    </nav>
  );
}
