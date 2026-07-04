import { oikumenea } from "@/lib/api/server";
import { ErrorNotice, PageHeader } from "@/components/ui";
import { DomainManager } from "@/components/DomainManager";
import { T } from "@/components/T";
import type { Domain } from "@/lib/api/types";

// The org-kind domain catalog (D-TenantOrganizations, M40): military / university / company / … — the
// classes that sit above organizations and scope unit-kinds. Instance-admin managed (domain.manage /
// unit-kind.manage). This is the admin view, so retired domains are shown too (unlike /organizations).
export default async function DomainsPage() {
  let domains: Domain[] = [];
  let error: unknown = null;
  try {
    const res = await oikumenea().then((ok) => ok.tenant.listDomains());
    domains = res.domains ?? [];
  } catch (e) {
    error = e;
  }

  return (
    <div className="max-w-3xl">
      <PageHeader
        title={<T>Domains & unit kinds</T>}
        description={<T>The org-kind catalog above organizations. Each domain scopes its own unit kinds — expand a domain to manage them.</T>}
      />
      {error ? <ErrorNotice error={error} /> : <DomainManager domains={domains} />}
    </div>
  );
}
