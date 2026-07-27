import Link from "next/link";
import { auth } from "@/auth";
import { Nav } from "@/components/Nav";
import { CommandPalette } from "@/components/CommandPalette";
import { SearchTrigger } from "@/components/SearchTrigger";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { signOutAction } from "@/lib/actions";
import { oikumenea } from "@/lib/api/server";
import { capabilities } from "@/lib/api/capabilities";
import { T } from "@/components/T";

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();
  const name = session?.user?.name ?? session?.user?.email ?? "signed in";
  // Populate the switcher from the instance-admin-managed locale list (falls back to eng/ukr).
  const localeList = await oikumenea()
    .then((ok) => ok.localization.listLocales())
    .catch(() => null);
  const supportedLocales = (localeList?.locales ?? [])
    .filter((l) => l.enabled !== false)
    .map((l) => ({ code: l.code, name: l.name }));

  // Resolve the caller's own effective permissions (D-SelfCapabilities) once, server-side; Nav and the
  // command palette are client components and filter their entries locally with it. Display-only — the
  // API re-decides on every call regardless of what the console offers.
  const caps = await capabilities();

  return (
    <div className="flex min-h-screen">
      <CommandPalette caps={caps} />
      <aside className="flex w-60 shrink-0 flex-col border-r border-slate-200 bg-white">
        <Link href="/" className="border-b border-slate-200 px-4 py-4">
          <div className="text-sm font-semibold text-slate-900">go-oikumenea</div>
          <div className="text-xs text-slate-400"><T msg="app.console" /></div>
        </Link>
        <div className="flex-1 overflow-y-auto">
          <Nav caps={caps} />
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center justify-between gap-3 border-b border-slate-200 bg-white px-6 py-3">
          <SearchTrigger />
          <div className="flex items-center gap-4">
            <LocaleSwitcher locales={supportedLocales} />
            <span className="text-sm text-slate-600">{name}</span>
            <form action={signOutAction}>
              <button type="submit" className="btn-ghost">
                <T msg="nav.signOut" />
              </button>
            </form>
          </div>
        </header>
        <main className="min-w-0 flex-1 overflow-y-auto p-6">{children}</main>
      </div>
    </div>
  );
}
