import type { Metadata } from "next";
import { cookies } from "next/headers";
import "./globals.css";
import { LocaleProvider } from "@/lib/locale";
import { setActiveLocale } from "@/lib/i18n";

export const metadata: Metadata = {
  title: "go-oikumenea — admin console",
  description: "Personnel & authorization admin console",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Seed the locale from the cookie so SSR renders the chosen locale (no flash, no mismatch).
  const initialLocale = (await cookies()).get("ui-locale")?.value;
  // Set the registry's module-global label locale for this request's server (RSC) render — server
  // components (e.g. the universal object view) resolve `locale → text` maps before LocaleProvider
  // (a client component) runs.
  setActiveLocale(initialLocale);
  return (
    <html lang={initialLocale === "ukr" ? "uk" : "en"}>
      <body>
        <LocaleProvider initialLocale={initialLocale}>{children}</LocaleProvider>
      </body>
    </html>
  );
}
