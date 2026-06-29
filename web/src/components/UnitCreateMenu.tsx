"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api/client";
import { T } from "./T";
import { useLocale } from "@/lib/locale";
import { pickLabel } from "@/lib/i18n";
import type { Domain } from "@/lib/api/types";

/**
 * Entry point for creating a unit: one button per org-kind domain (military / university / …) instead
 * of a single "New unit" form that asks which kind. The domain is chosen here and carried into
 * /units/new?domain=<rid>, where the create form is pre-scoped to it (the domain is no longer a field).
 */
export function UnitCreateMenu() {
  const { locale } = useLocale();
  const [domains, setDomains] = useState<Domain[]>([]);

  useEffect(() => {
    let alive = true;
    api.tenant
      .listDomains()
      .then((d) => {
        if (alive) setDomains((d.domains ?? []).filter((x) => x.status !== "retired"));
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  if (domains.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-2">
      {domains.map((d) => (
        <Link key={d.id} href={`/units/new?domain=${encodeURIComponent(d.id)}`} className="btn-ghost">
          <T>New</T> <span className="lowercase">{pickLabel(d.name, locale) || d.code}</span> <T>unit</T>
        </Link>
      ))}
    </div>
  );
}
