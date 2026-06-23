"use client";

import { createContext, useContext, useState } from "react";
import { DEFAULT_LOCALE, setActiveLocale } from "./i18n";
import { t, tg } from "./messages";

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
  // Keep the registry's module-global label locale in sync. Set in render (covers the client SSR/HTML
  // pass) and on every switch, so name accessors resolve against the chosen locale. The provider is an
  // ancestor of every label-rendering view, so it runs first on each re-render.
  setActiveLocale(locale);
  const setLocale = (l: string) => {
    setActiveLocale(l);
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

/**
 * Client hook for static-chrome translation: returns a `t(key)` bound to the current UI locale, so
 * components re-render and re-translate on switch. Server components call `t(key, getActiveLocale())`
 * from `@/lib/messages` directly instead.
 */
export const useT = () => {
  const { locale } = useLocale();
  return (key: string) => t(key, locale);
};

/**
 * Client hook for glossary translation by English source string (the `tg()` counterpart of `useT`).
 * Returns a `tr(text)` bound to the current UI locale — use it in client components for string props
 * (placeholders, aria labels) where a `<T>` element can't go.
 */
export const useTg = () => {
  const { locale } = useLocale();
  return (text: string) => tg(text, locale);
};
