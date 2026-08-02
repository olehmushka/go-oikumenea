import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { ErrorNotice, PageHeader } from "@/components/ui";
import { OrganizationManager } from "@/components/OrganizationManager";
import { T } from "@/components/T";
import type { Domain, Organization } from "@/lib/api/types";

// Organization administration (tenant): the realms a person joins (US Army / KhNU / …). An org is
// required before any unit can be created, so this is where you stand one up. Each org belongs to a
// domain (its kind) and seeds its command + operational graphs at creation (D-TenantOrganizations, M40).
//
// M58 ticket 4 moved BROWSING out. /explore/organization is the registry's real reader: the domain /
// visibility / state filters, keyset paging that does not drop its token, and a dashboard over the
// same filter set. What stays here is EDITING — creation, domain assignment and the lifecycle
// transitions, which are richer than the generic action runner.
//
// The fetch below is therefore a bounded EDIT surface, not a listing: it takes one page and REPORTS
// truncation rather than silently dropping the next-page token, which is what the pre-M58 `200` did.
const EDIT_PAGE = 50;

export default async function OrganizationsPage() {
  let organizations: Organization[] = [];
  let domains: Domain[] = [];
  let truncated = false;
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    const [orgsRes, domainsRes] = await Promise.all([
      ok.tenant.listOrganizations(undefined, undefined, undefined, EDIT_PAGE),
      ok.tenant.listDomains(),
    ]);
    organizations = orgsRes.organizations ?? [];
    truncated = Boolean(orgsRes.nextPageToken);
    domains = (domainsRes.domains ?? []).filter((d) => d.status !== "retired");
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title={<T>Organizations</T>}
        description={<T>The realms a person joins. Create one (and pick its domain) before adding units.</T>}
      />
      <div className="mb-3 flex items-center gap-3">
        <p className="text-xs text-slate-500">
          {truncated ? (
            <T>The first page only — there are more. Use the explorer to find a specific organization; this table is here to edit the ones in front of you.</T>
          ) : (
            <T>Every organization in the registry. Use the explorer to filter or chart them.</T>
          )}
        </p>
        <Link href="/explore/organization" className="ml-auto text-xs text-indigo-600 hover:underline">
          <T>Browse, filter and chart every organization →</T>
        </Link>
      </div>
      {error ? <ErrorNotice error={error} /> : <OrganizationManager organizations={organizations} domains={domains} />}
    </div>
  );
}
