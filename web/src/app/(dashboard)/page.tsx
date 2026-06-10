import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import { Card, ErrorNotice, Mono, PageHeader } from "@/components/ui";
import { RecentsPanel } from "@/components/ontology/RecentsPanel";
import type { Whoami, VersionInfo } from "@/lib/api/types";

export default async function OverviewPage() {
  let whoami: Whoami | null = null;
  let version: VersionInfo | null = null;
  let error: unknown = null;
  try {
    [whoami, version] = await Promise.all([
      apiGet<Whoami>("/identity/v1/whoami"),
      apiGet<VersionInfo>("/platform/v1/status/version").catch(() => null),
    ]);
  } catch (e) {
    error = e;
  }

  const links = [
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
        title="Overview"
        description="Press ⌘K anywhere to search objects, jump to a view, or paste a RID."
      />
      {error ? (
        <ErrorNotice error={error} />
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          <Card>
            <h2 className="text-sm font-semibold text-slate-900">Signed in as</h2>
            <dl className="mt-3 space-y-2 text-sm">
              <Row label="Email" value={whoami?.email} />
              <Row
                label="Person"
                value={whoami?.personId ? <Mono>{whoami.personId}</Mono> : undefined}
              />
              <Row
                label="Account"
                value={
                  whoami?.accountId ? <Mono>{whoami.accountId}</Mono> : undefined
                }
              />
            </dl>
            <p className="mt-4 text-xs text-slate-400">
              Authentication is delegated to Keycloak; the service resolved this token to
              the person above and decides authorization per request (the PDP).
            </p>
          </Card>

          <Card>
            <h2 className="text-sm font-semibold text-slate-900">Service</h2>
            <dl className="mt-3 space-y-2 text-sm">
              <Row label="Version" value={version?.version ?? "—"} />
              <Row label="Schema" value={version?.schemaVersion ?? "—"} />
            </dl>
          </Card>
        </div>
      )}

      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900">Workspace</h2>
      <RecentsPanel />

      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900">Jump to</h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {links.map(([href, title, hint]) => (
          <Link key={href} href={href} className="card p-4 hover:bg-slate-50">
            <div className="text-sm font-medium text-slate-900">{title}</div>
            <div className="text-xs text-slate-500">{hint}</div>
          </Link>
        ))}
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value?: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-slate-500">{label}</dt>
      <dd className="text-right text-slate-800">{value ?? "—"}</dd>
    </div>
  );
}
