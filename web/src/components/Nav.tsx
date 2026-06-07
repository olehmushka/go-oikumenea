"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const ITEMS: { href: string; label: string; hint: string }[] = [
  { href: "/", label: "Overview", hint: "whoami & reach" },
  { href: "/units", label: "Units", hint: "tenant DAG" },
  { href: "/persons", label: "Persons", hint: "directory" },
  { href: "/memberships", label: "Memberships", hint: "positions" },
  { href: "/ranks", label: "Ranks", hint: "rank scheme" },
  { href: "/roles", label: "Roles & access", hint: "RBAC" },
  { href: "/authorize", label: "Authorize", hint: "PDP check" },
  { href: "/documents", label: "Documents", hint: "papers & codes" },
  { href: "/orders", label: "Orders", hint: "наказ" },
  { href: "/localization", label: "Localization", hint: "locales" },
  { href: "/audit", label: "Audit", hint: "log" },
];

export function Nav() {
  const pathname = usePathname();
  return (
    <nav className="flex flex-col gap-0.5 p-3">
      {ITEMS.map((it) => {
        const active =
          it.href === "/" ? pathname === "/" : pathname.startsWith(it.href);
        return (
          <Link
            key={it.href}
            href={it.href}
            className={`flex flex-col rounded-md px-3 py-2 ${
              active
                ? "bg-indigo-50 text-indigo-700"
                : "text-slate-700 hover:bg-slate-100"
            }`}
          >
            <span className="text-sm font-medium">{it.label}</span>
            <span className="text-xs text-slate-400">{it.hint}</span>
          </Link>
        );
      })}
    </nav>
  );
}
