"use client";

import { createContext, useContext, useState } from "react";
import { DEFAULT_LOCALE } from "./i18n";

/**
 * Client-side UI-locale state (label rendering only; the API always returns every locale).
 *
 * The chosen locale is persisted in the `ui-locale` **cookie** so the SERVER can read it and
 * render the correct locale on every request (every page here is a server component). The provider
 * is seeded with `initialLocale` (read from that cookie in the root layout), so SSR and the client
 * agree — no flash, and switching is reliable. The setter writes the cookie; the switcher then
 * calls router.refresh() so server-rendered content re-renders in the new locale.
 */
const LocaleContext = createContext<{
  locale: string;
  setLocale: (l: string) => void;
}>({ locale: DEFAULT_LOCALE, setLocale: () => {} });

export function LocaleProvider({
  children,
  initialLocale,
}: {
  children: React.ReactNode;
  initialLocale?: string;
}) {
  const [locale, setLocaleState] = useState<string>(initialLocale || DEFAULT_LOCALE);
  const setLocale = (l: string) => {
    setLocaleState(l);
    try {
      // 1 year; lax is fine (same-site BFF). Server reads this in the root layout.
      document.cookie = `ui-locale=${encodeURIComponent(l)}; path=/; max-age=31536000; samesite=lax`;
      window.localStorage.setItem("ui-locale", l);
    } catch {
      /* SSR / storage disabled — ignore */
    }
  };
  return (
    <LocaleContext.Provider value={{ locale, setLocale }}>{children}</LocaleContext.Provider>
  );
}

export const useLocale = () => useContext(LocaleContext);
