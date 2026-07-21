import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { can } from "@/lib/api/can";
import {
  EmptyState,
  ErrorNotice,
  Mono,
  PageHeader,
  Pill,
  Table,
} from "@/components/ui";
import { T } from "@/components/T";
import {
  EditPrincipal,
  GrantPermission,
  PrincipalRegister,
  PrincipalStatusToggle,
  RevokeGrant,
} from "./PrincipalForms";
import type {
  IssuerOption,
  PrincipalGrantPage,
  ServicePrincipalPage,
} from "@/lib/api/types";

/**
 * The service-principal console (M52) — machine subjects (M51 / D-ServiceIdentities) and their flat
 * per-principal grants. `service-principal.read` / `.manage` are instance-scope and sit in no base
 * role, so in practice only instance admins reach this page.
 */
export default async function ServicePrincipalsPage({
  searchParams,
}: {
  searchParams: Promise<{ principalId?: string }>;
}) {
  const { principalId } = await searchParams;

  // Gate the deep link too, not just the nav entry. Display-only — the endpoints below re-decide
  // server-side regardless (see lib/api/can.ts).
  if (!(await can("service-principal.read"))) {
    return (
      <div>
        <PageHeader title={<T>Service principals</T>} />
        <EmptyState>
          <T>
            This surface is limited to instance administrators (service-principal.read).
          </T>
        </EmptyState>
      </div>
    );
  }

  let principals: ServicePrincipalPage | null = null;
  let issuers: IssuerOption[] = [];
  let grants: PrincipalGrantPage | null = null;
  let error: unknown = null;
  let grantError: unknown = null;

  try {
    principals = await oikumenea().then((ok) =>
      ok.identityFederation.listServicePrincipals(100),
    );
  } catch (e) {
    error = e;
  }
  // Non-fatal: the register form falls back to a free-text issuer field without this.
  try {
    issuers = await oikumenea().then((ok) => ok.identityFederation.listIssuers());
  } catch {
    issuers = [];
  }
  // listPrincipalGrants REQUIRES principalId — there is no global grant list, so grants render as
  // child rows under a selected principal.
  if (principalId) {
    try {
      grants = await oikumenea().then((ok) =>
        ok.authorization.listPrincipalGrants(principalId),
      );
    } catch (e) {
      grantError = e;
    }
  }

  const selected = (principals?.principals ?? []).find((p) => p.id === principalId);

  return (
    <div>
      <PageHeader
        title={<T>Service principals</T>}
        description={
          <T>
            Machine subjects that authenticate through the same external IdP as a person, via the
            OAuth2 client-credentials grant. A principal holds no role assignment and no unit reach —
            its authority is the flat per-principal grants below. Registering one creates no
            credential: the IdP owns the client secret.
          </T>
        }
      />
      {error ? <ErrorNotice error={error} /> : null}

      {/* Principals */}
      <h2 className="mb-3 text-sm font-semibold text-slate-900">
        <T>Principals</T>
      </h2>
      {principals && principals.principals.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Code</T></th>
              <th className="th"><T>Name</T></th>
              <th className="th"><T>Issuer</T></th>
              <th className="th"><T>Subject</T></th>
              <th className="th"><T>Client</T></th>
              <th className="th"><T>Status</T></th>
              <th className="th"></th>
            </>
          }
        >
          {principals.principals.map((p) => (
            <tr key={p.id} className={p.id === principalId ? "bg-indigo-50/50" : undefined}>
              <td className="td">
                <Mono>{p.code}</Mono>
              </td>
              <td className="td">{p.name}</td>
              <td className="td">
                <Mono>{p.issuer}</Mono>
              </td>
              <td className="td">
                <Mono>{p.subject.slice(-12)}</Mono>
              </td>
              <td className="td">{p.clientId ? <Mono>{p.clientId}</Mono> : "—"}</td>
              <td className="td">
                <Pill tone={p.status === "active" ? "indigo" : "slate"}>{p.status}</Pill>
              </td>
              <td className="td text-right">
                <span className="relative inline-flex items-center gap-3">
                  <Link
                    href={`/service-principals?principalId=${encodeURIComponent(p.id)}`}
                    className="text-xs font-medium text-indigo-600 hover:underline"
                  >
                    Grants
                  </Link>
                  <EditPrincipal principal={p} />
                  <PrincipalStatusToggle principal={p} />
                </span>
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState><T>No service principals.</T></EmptyState>
      )}
      <div className="mt-4">
        <PrincipalRegister issuers={issuers} />
      </div>

      {/* Grants */}
      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900">
        <T>Grants</T>
        {selected ? (
          <span className="ml-2 font-normal text-slate-500">
            — <Mono>{selected.code}</Mono>
          </span>
        ) : null}
      </h2>
      {grantError ? <ErrorNotice error={grantError} /> : null}
      {!principalId ? (
        <EmptyState>
          <T>Pick a principal (Grants) to list the permissions it holds.</T>
        </EmptyState>
      ) : grants && grants.grants.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Permission</T></th>
              <th className="th"><T>Organization</T></th>
              <th className="th"><T>Granted</T></th>
              <th className="th"></th>
            </>
          }
        >
          {grants.grants.map((g) => (
            <tr key={g.id}>
              <td className="td">
                <Mono>{g.permission}</Mono>
              </td>
              <td className="td">
                {g.orgId ? (
                  <Mono>{g.orgId.slice(-8)}</Mono>
                ) : (
                  <Pill tone="indigo"><T>instance-wide</T></Pill>
                )}
              </td>
              <td className="td">{g.grantedAt}</td>
              <td className="td text-right">
                <RevokeGrant grantId={g.id} permission={g.permission} />
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState><T>No active grants for this principal.</T></EmptyState>
      )}
      {principalId ? (
        <div className="mt-4">
          <GrantPermission principalId={principalId} />
        </div>
      ) : null}
    </div>
  );
}
