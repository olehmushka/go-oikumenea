"use client";

import { useRouter } from "next/navigation";
import { useLocale } from "@/lib/locale";
import { LOCALE_LABELS } from "@/lib/i18n";

const FALLBACK = [
  { code: "eng", name: "English" },
  { code: "ukr", name: "Українська" },
];

/**
 * Switches the UI label locale. The supported list comes from the localization module
 * (`/localization/v1/locales`, passed in by the dashboard layout); on change it stores the choice
 * (cookie, via setLocale) and calls router.refresh() so server components re-render in the new
 * locale. The API itself always returns every locale (D-i18n) — this only picks which to show.
 */
export function LocaleSwitcher({ locales }: { locales?: { code: string; name?: string }[] }) {
  const { locale, setLocale } = useLocale();
  const router = useRouter();
  const list = locales && locales.length > 0 ? locales : FALLBACK;
  return (
    <label className="flex items-center gap-1 text-xs text-slate-500">
      <span className="hidden sm:inline">Locale</span>
      <select
        className="rounded-md border border-slate-300 bg-white px-2 py-1 text-sm text-slate-700"
        value={locale}
        onChange={(e) => {
          setLocale(e.target.value);
          router.refresh();
        }}
      >
        {list.map((l) => (
          <option key={l.code} value={l.code}>
            {l.name || LOCALE_LABELS[l.code] || l.code}
          </option>
        ))}
      </select>
    </label>
  );
}
