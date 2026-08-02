import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { PageHeader, EmptyState, ErrorNotice } from "@/components/ui";
import { NewUnitForm, type Opt } from "@/components/NewUnitForm";
import { UnitCreateMenu } from "@/components/UnitCreateMenu";
import { T } from "@/components/T";
import { pickLabel, getActiveLocale } from "@/lib/i18n";

// Creating a unit is now domain-first: the user arrives here with ?domain=<rid> chosen from the
// per-domain buttons (UnitCreateMenu). The domain is fixed (not a form field); the form only collects
// what varies within a domain (org, kind, name/code, parent). Without a domain we show the chooser.
export default async function NewUnitPage({
  searchParams,
}: {
  searchParams: Promise<{ domain?: string }>;
}) {
  const { domain } = await searchParams;
  const locale = getActiveLocale();

  if (!domain) {
    return (
      <div className="max-w-lg">
        <PageHeader title={<T>New unit</T>} description={<T>First pick the kind of organization the unit belongs to.</T>} />
        <UnitCreateMenu />
      </div>
    );
  }

  let domainLabel = domain;
  let orgs: Opt[] = [];
  let kinds: Opt[] = [];
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    const [domainsRes, orgsRes, kindsRes] = await Promise.all([
      ok.tenant.listDomains(),
      ok.tenant.listOrganizations(domain, undefined, undefined, 200),
      ok.tenant.listUnitKinds(domain),
    ]);
    const dom = (domainsRes.domains ?? []).find((d) => d.id === domain);
    domainLabel = dom ? pickLabel(dom.name, locale) || dom.code : domain;
    orgs = (orgsRes.organizations ?? []).map((o) => ({
      id: o.id,
      label: pickLabel(o.name, locale) || o.code,
    }));
    kinds = (kindsRes.unitKinds ?? []).map((k) => ({
      id: k.id,
      label: pickLabel(k.name, locale) || k.code,
    }));
  } catch (e) {
    error = e;
  }

  return (
    <div className="max-w-lg">
      <PageHeader
        title={<><T>New</T> <span className="lowercase">{domainLabel}</span> <T>unit</T></>}
        description={<T>Create a unit in this domain. The domain is fixed by the kind you chose; pick the organization it belongs to.</T>}
      />
      {error ? <div className="mb-4"><ErrorNotice error={error} /></div> : null}
      {!error && orgs.length === 0 ? (
        <EmptyState>
          <T>This domain has no organizations yet.</T>{" "}
          <Link href="/organizations" className="font-medium text-indigo-600 hover:underline">
            <T>Create an organization first.</T>
          </Link>
        </EmptyState>
      ) : null}
      {!error && orgs.length > 0 ? (
        <NewUnitForm domainId={domain} domainLabel={domainLabel} orgs={orgs} kinds={kinds} />
      ) : null}
    </div>
  );
}
