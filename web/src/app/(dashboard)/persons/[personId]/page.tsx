import { Suspense } from "react";
import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import { Card, EmptyState, ErrorNotice, Mono, PageHeader, Pill } from "@/components/ui";
import {
  CallSignManager,
  CitizenshipManager,
  DocumentManager,
  EditPerson,
  EmailManager,
  MessengerLinkManager,
  NameVariantManager,
  PersonalCodeManager,
  PersonLanguageManager,
  PersonLifecycle,
  PersonRankLabel,
  PhoneManager,
  RelationshipManager,
  ResidenceManager,
  SetRank,
  SocialAccountManager,
} from "./PersonForms";
import { PersonEducationManager } from "./PersonEducation";
import type {
  Association,
  DocumentDoc,
  Guardianship,
  Kinship,
  Membership,
  NextOfKin,
  Order,
  Partnership,
  Person,
  Sponsorship,
} from "@/lib/api/types";

type CodeRow = { id: string; schemeCode?: string; status?: string };

export default async function PersonDetailPage({
  params,
}: {
  params: Promise<{ personId: string }>;
}) {
  const { personId } = await params;
  let person: Person | null = null;
  let error: unknown = null;
  try {
    person = await apiGet<Person>(`/person/v1/persons/${personId}`);
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
            <Row label="Rank" value={<PersonRankLabel ranks={person.ranks} />} />
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
            <SetRank personId={person.id} ranks={person.ranks} />
          </div>
          <div className="mt-3 border-t border-slate-100 pt-3">
            <PersonLifecycle person={person} />
          </div>
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Contact channels</h2>
          <EmailManager personId={person.id} emails={person.emails} />
          <PhoneManager personId={person.id} phones={person.phones} />
          <CallSignManager personId={person.id} callSigns={person.callSigns} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Social &amp; messenger</h2>
          <SocialAccountManager personId={person.id} accounts={person.socialAccounts} />
          <MessengerLinkManager
            personId={person.id}
            links={person.messengerLinks}
            emails={person.emails}
            phones={person.phones}
          />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Languages</h2>
          <PersonLanguageManager personId={person.id} />
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
      </div>

      <Suspense fallback={<StreamFallback />}>
        <PersonRelations personId={person.id} person={person} />
      </Suspense>
    </div>
  );
}

// PersonRelations holds everything that needs a second round of per-person reads (documents, codes,
// relationships, memberships, orders). It is streamed under <Suspense> so the page shell paints as
// soon as the person resolves — the create→navigate path no longer blocks on this batch.
async function PersonRelations({ personId, person }: { personId: string; person: Person }) {
  const [documents, codes, memberships, orders, partnerships, kinships, guardianships, sponsorships, nextOfKin, associations] =
    await Promise.all([
      apiGet<{ documents: DocumentDoc[] }>(`/document/v1/persons/${personId}/documents`).catch(() => null),
      apiGet<{ codes?: CodeRow[] }>(`/document/v1/persons/${personId}/personal-codes`).catch(() => null),
      apiGet<{ memberships: Membership[] }>(`/membership/v1/persons/${personId}/memberships`).catch(() => null),
      apiGet<{ orders: Order[] }>(`/order/v1/persons/${personId}/orders`).catch(() => null),
      apiGet<Partnership[]>(`/person/v1/persons/${personId}/partnerships`).catch(() => []),
      apiGet<Kinship[]>(`/person/v1/persons/${personId}/kinships`).catch(() => []),
      apiGet<Guardianship[]>(`/person/v1/persons/${personId}/guardianships`).catch(() => []),
      apiGet<Sponsorship[]>(`/person/v1/persons/${personId}/sponsorships`).catch(() => []),
      apiGet<NextOfKin[]>(`/person/v1/persons/${personId}/next-of-kin`).catch(() => []),
      apiGet<Association[]>(`/person/v1/persons/${personId}/associations`).catch(() => []),
    ]);

  return (
    <>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900">Relationships</h2>
          <RelationshipManager
            personId={person.id}
            partnerships={partnerships}
            kinships={kinships}
            guardianships={guardianships}
            sponsorships={sponsorships}
            nextOfKin={nextOfKin}
            associations={associations}
          />
        </Card>

        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900">Documents &amp; personal codes</h2>
          <DocumentManager personId={person.id} documents={documents?.documents} />
          <PersonalCodeManager personId={person.id} codes={codes?.codes} />
        </Card>

        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900">Education</h2>
          <PersonEducationManager personId={person.id} />
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
    </>
  );
}

function StreamFallback() {
  return <div className="mt-4 text-sm text-slate-400">Loading relationships, documents, memberships…</div>;
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
