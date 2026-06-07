"use client";

import { useLocale } from "@/lib/locale";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

/** Renders a `locale → text` map using the current UI locale (D-i18n), with fallback. */
export function Localized({ map, fallback }: { map: LocaleMap; fallback?: string }) {
  const { locale } = useLocale();
  const text = pickLabel(map, locale);
  return <>{text || fallback || <span className="text-slate-400">—</span>}</>;
}
