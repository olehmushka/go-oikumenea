import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { Card, ErrorNotice, Mono, PageHeader } from "@/components/ui";
import { RecentsPanel } from "@/components/ontology/RecentsPanel";
import { T } from "@/components/T";
import type { Whoami, VersionInfo } from "@/lib/api/types";

export default async function OverviewPage() {
  let whoami: Whoami | null = null;
  let version: VersionInfo | null = null;
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    [whoami, version] = await Promise.all([
      ok.identityFederation.whoami(),
      ok.platform.version().catch(() => null),
    ]);
  } catch (e) {
    error = e;
  }

  const links: [string, string, string][] = [
    ["/explore/person", "Persons", "The personnel directory"],
    ["/explore/unit", "Units", "Browse the unit DAG"],
    ["/explore/role", "Roles", "RBAC roles"],
    ["/ontology", "Ontology", "The type registry"],
    ["/authorize", "Authorize", "Run a PDP decision"],
    ["/audit", "Audit", "Permission-sensitive log"],
  ];

  return (
    <div>
      <PageHeader
        title={<T>Overview</T>}
        description={<T>Press ⌘K anywhere to search objects, jump to a view, or paste a RID.</T>}
      />
      {error ? (
        <ErrorNotice error={error} />
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <h2 className="text-sm font-semibold text-slate-900"><T>Signed in as</T></h2>
            <dl className="mt-3 space-y-2 text-sm">
              <Row label={<T>Email</T>} value={whoami?.email} />
              <Row
                label={<T>Person</T>}
                value={whoami?.personId ? <Mono>{whoami.personId}</Mono> : undefined}
              />
              <Row
                label={<T>Account</T>}
                value={
                  whoami?.accountId ? <Mono>{whoami.accountId}</Mono> : undefined
                }
              />
            </dl>
            <p className="mt-4 text-xs text-slate-400">
              <T>Authentication is delegated to Keycloak; the service resolved this token to the person above and decides authorization per request (the PDP).</T>
            </p>
          </Card>

          <Card>
            <h2 className="text-sm font-semibold text-slate-900"><T>Service</T></h2>
            <dl className="mt-3 space-y-2 text-sm">
              <Row label={<T>Version</T>} value={version?.binaryRevision ?? "—"} />
              <Row label={<T>Schema</T>} value={version?.schemaRevision ?? "—"} />
            </dl>
          </Card>
        </div>
      )}

      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900"><T>Workspace</T></h2>
      <RecentsPanel />

      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900"><T>Jump to</T></h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {links.map(([href, title, hint]) => (
          <Link key={href} href={href} className="card p-4 hover:bg-slate-50">
            <div className="text-sm font-medium text-slate-900"><T>{title}</T></div>
            <div className="text-xs text-slate-500"><T>{hint}</T></div>
          </Link>
        ))}
      </div>
    </div>
  );
}

function Row({ label, value }: { label: React.ReactNode; value?: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-slate-500">{label}</dt>
      <dd className="text-right text-slate-800">{value ?? "—"}</dd>
    </div>
  );
}
