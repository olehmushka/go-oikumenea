import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import { Card, EmptyState, ErrorNotice, Mono, PageHeader, Pill } from "@/components/ui";
import {
  CallSignManager,
  CitizenshipManager,
  DocumentManager,
  EditPerson,
  EmailManager,
  NameVariantManager,
  PersonalCodeManager,
  PersonLifecycle,
  PhoneManager,
  ResidenceManager,
  SetRank,
} from "./PersonForms";
import type {
  DocumentDoc,
  DocumentType,
  LocaleMap,
  Membership,
  Order,
  PersonalCodeScheme,
  Person,
} from "@/lib/api/types";

type ContactType = { code: string; name?: LocaleMap };
type CodeRow = { id: string; schemeCode?: string; status?: string };

export default async function PersonDetailPage({
  params,
}: {
  params: Promise<{ personId: string }>;
}) {
  const { personId } = await params;
  let person: Person | null = null;
  let documents: { documents: DocumentDoc[] } | null = null;
  let codes: { codes?: CodeRow[] } | null = null;
  let memberships: { memberships: Membership[] } | null = null;
  let orders: { orders: Order[] } | null = null;
  let emailTypes: ContactType[] = [];
  let phoneTypes: ContactType[] = [];
  let docTypes: DocumentType[] = [];
  let schemes: PersonalCodeScheme[] = [];
  let error: unknown = null;
  try {
    person = await apiGet<Person>(`/person/v1/persons/${personId}`);
    [documents, codes, memberships, orders, emailTypes, phoneTypes, docTypes, schemes] =
      await Promise.all([
        apiGet<{ documents: DocumentDoc[] }>(`/document/v1/persons/${personId}/documents`).catch(
          () => null,
        ),
        apiGet<{ codes?: CodeRow[] }>(`/document/v1/persons/${personId}/personal-codes`).catch(
          () => null,
        ),
        apiGet<{ memberships: Membership[] }>(
          `/membership/v1/persons/${personId}/memberships`,
        ).catch(() => null),
        apiGet<{ orders: Order[] }>(`/order/v1/persons/${personId}/orders`).catch(() => null),
        apiGet<ContactType[]>("/person/v1/person/email-types").catch(() => []),
        apiGet<ContactType[]>("/person/v1/person/phone-types").catch(() => []),
        apiGet<DocumentType[]>("/document/v1/document-types").catch(() => []),
        apiGet<PersonalCodeScheme[]>("/document/v1/personal-code-schemes").catch(() => []),
      ]);
  } catch (e) {
    error = e;
  }

  if (error || !person) {
    return (
      <div>
        <PageHeader title="Person" />
        <ErrorNotice error={error} />
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title={person.displayName ?? personId}
        description={person.code ? `code ${person.code}` : undefined}
        action={
          <Link href="/persons" className="btn-ghost">
            ← All persons
          </Link>
        }
      />

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <div className="flex items-start justify-between">
            <h2 className="text-sm font-semibold text-slate-900">Identity</h2>
            <EditPerson person={person} />
          </div>
          <dl className="mt-3 space-y-2 text-sm">
            <Row label="Given" value={person.given} />
            <Row label="Surname" value={person.surname} />
            <Row label="Birthdate" value={person.birthdate} />
            <Row label="Sex" value={person.sex} />
            <Row label="Country of birth" value={person.countryOfBirth} />
            <Row
              label="Status"
              value={
                <Pill tone={(person.status ?? "").toUpperCase() === "ACTIVE" ? "green" : "slate"}>
                  {person.status ?? "—"}
                </Pill>
              }
            />
            <Row label="ID" value={<Mono>{person.id}</Mono>} />
          </dl>
          <div className="mt-4 border-t border-slate-100 pt-3">
            <SetRank personId={person.id} currentRankId={person.rankId} />
          </div>
          <div className="mt-3 border-t border-slate-100 pt-3">
            <PersonLifecycle person={person} />
          </div>
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Contact channels</h2>
          <EmailManager personId={person.id} emails={person.emails} types={emailTypes} />
          <PhoneManager personId={person.id} phones={person.phones} types={phoneTypes} />
          <CallSignManager personId={person.id} callSigns={person.callSigns} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Citizenship &amp; residence</h2>
          <CitizenshipManager personId={person.id} citizenships={person.citizenships} />
          <ResidenceManager personId={person.id} residences={person.residences} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Name variants</h2>
          <NameVariantManager personId={person.id} variants={person.nameVariants} />
        </Card>

        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900">Documents &amp; personal codes</h2>
          <DocumentManager
            personId={person.id}
            documents={documents?.documents}
            types={docTypes}
          />
          <PersonalCodeManager personId={person.id} codes={codes?.codes} schemes={schemes} />
        </Card>
      </div>

      <Section title="Memberships">
        {memberships?.memberships?.length ? (
          <ul className="space-y-1 text-sm">
            {memberships.memberships.map((m) => (
              <li key={m.id} className="flex items-center gap-2">
                <Link href={`/units/${m.unitId}`} className="text-indigo-600 hover:underline">
                  <Mono>{m.unitId.slice(-8)}</Mono>
                </Link>
                <Pill tone={m.status === "ACTIVE" ? "green" : "slate"}>{m.status ?? "—"}</Pill>
                <span className="text-slate-400">{m.effectiveFrom ?? ""}</span>
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState>No memberships.</EmptyState>
        )}
      </Section>

      <Section title="Orders">
        {orders?.orders?.length ? (
          <ul className="space-y-1 text-sm">
            {orders.orders.map((o) => (
              <li key={o.id}>
                <Link href={`/orders/${o.id}`} className="text-indigo-600 hover:underline">
                  <Mono>{o.number ?? o.id.slice(-8)}</Mono>
                </Link>{" "}
                <Pill tone={o.status === "ISSUED" ? "green" : "slate"}>{o.status ?? "—"}</Pill>
              </li>
            ))}
          </ul>
        ) : (
          <EmptyState>No orders reference this person.</EmptyState>
        )}
      </Section>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mt-8">
      <h2 className="mb-3 text-sm font-semibold text-slate-900">{title}</h2>
      {children}
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
