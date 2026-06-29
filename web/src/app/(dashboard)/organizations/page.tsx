import { oikumenea } from "@/lib/api/server";
import { ErrorNotice, PageHeader } from "@/components/ui";
import { OrganizationManager } from "@/components/OrganizationManager";
import { T } from "@/components/T";
import type { Domain, Organization } from "@/lib/api/types";

// Organization administration (tenant): the realms a person joins (US Army / KhNU / …). An org is
// required before any unit can be created, so this is where you stand one up. Each org belongs to a
// domain (its kind) and seeds its command + operational graphs at creation (D-TenantOrganizations, M40).
export default async function OrganizationsPage() {
  let organizations: Organization[] = [];
  let domains: Domain[] = [];
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    const [orgsRes, domainsRes] = await Promise.all([
      ok.tenant.listOrganizations(undefined, 200),
      ok.tenant.listDomains(),
    ]);
    organizations = orgsRes.organizations ?? [];
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
      {error ? <ErrorNotice error={error} /> : <OrganizationManager organizations={organizations} domains={domains} />}
    </div>
  );
}
