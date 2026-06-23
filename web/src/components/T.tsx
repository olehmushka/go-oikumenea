"use client";

import { useLocale } from "@/lib/locale";
import { tg, t as tmsg } from "@/lib/messages";

/**
 * Client leaf that translates a static-chrome string at *render time* on the client, from a stable
 * English source. Use it for UI chrome rendered by SERVER components (page titles, section headings,
 * registry type/label text): a server component bakes a `tg()`/`t()` result into its RSC payload, so
 * it would not follow the live UI locale (and is subject to the router cache) — whereas `<T>` is a
 * client island whose only prop is the locale-independent English key, so it re-renders in the chosen
 * locale on switch and stays correct even when the surrounding server payload is cached.
 *
 * Pass the English text as children for the glossary (`<T>Persons</T>`), or a message-catalog key via
 * `msg` (`<T msg="nav.signOut" />`). Unknown strings fall back to the English source.
 */
export function T({ children, msg }: { children?: string; msg?: string }) {
  const { locale } = useLocale();
  return <>{msg != null ? tmsg(msg, locale) : tg(children ?? "", locale)}</>;
}
