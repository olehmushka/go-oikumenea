import { Suspense } from "react";
import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { Card, EmptyState, ErrorNotice, Mono, PageHeader, Pill } from "@/components/ui";
import { T } from "@/components/T";
import {
  CallSignManager,
  CitizenshipManager,
  DocumentManager,
  EditPerson,
  MergeProvisional,
  EmailManager,
  MessengerLinkManager,
  NameVariantManager,
  PersonAffiliationManager,
  PersonalCodeManager,
  PersonClergyManager,
  PersonLanguageManager,
  PersonLifecycle,
  PersonRankLabel,
  PhoneManager,
  PhysicalIdentityManager,
  RelationshipManager,
  ResidenceManager,
  SetRank,
  SocialAccountManager,
} from "./PersonForms";
import { PersonEducationManager } from "./PersonEducation";
import { PersonCompaniesManager } from "./PersonCompanies";
import { PersonVehiclesManager } from "./PersonVehicles";
import { AccountManager } from "./PersonAccount";
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
    person = await oikumenea().then((ok) => ok.person.getPerson(personId));
  } catch (e) {
    error = e;
  }

  if (error || !person) {
    return (
      <div>
        <PageHeader title={<T>Person</T>} />
        <ErrorNotice error={error} />
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title={person.displayName ?? personId}
        description={person.code ? <><T>code</T> {person.code}</> : undefined}
        action={
          <Link href="/persons" className="btn-ghost">
            ← <T>All persons</T>
          </Link>
        }
      />

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <div className="flex items-start justify-between">
            <h2 className="text-sm font-semibold text-slate-900"><T>Identity</T></h2>
            <EditPerson person={person} />
          </div>
          <dl className="mt-3 space-y-2 text-sm">
            <Row label={<T>Given</T>} value={person.given} />
            <Row label={<T>Surname</T>} value={person.surname} />
            <Row label={<T>Birthdate</T>} value={person.birthdate} />
            <Row label={<T>Date of death</T>} value={person.dateOfDeath} />
            <Row label={<T>Sex</T>} value={person.sex} />
            <Row label={<T>Rank</T>} value={<PersonRankLabel ranks={person.ranks} />} />
            <Row label={<T>Country of birth</T>} value={person.countryOfBirth} />
            <Row
              label={<T>Status</T>}
              value={
                <Pill tone={(person.status ?? "").toUpperCase() === "ACTIVE" ? "green" : "slate"}>
                  {person.status ?? "—"}
                </Pill>
              }
            />
            <Row label={<T>ID</T>} value={<Mono>{person.id}</Mono>} />
          </dl>
          <div className="mt-4 border-t border-slate-100 pt-3">
            <SetRank personId={person.id} ranks={person.ranks} />
          </div>
          <div className="mt-3 border-t border-slate-100 pt-3">
            <PersonLifecycle person={person} />
          </div>
          {(person.status ?? "").toLowerCase() === "provisional" ? (
            <div className="mt-3 border-t border-slate-100 pt-3">
              <MergeProvisional person={person} />
            </div>
          ) : null}
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Contact channels</T></h2>
          <EmailManager personId={person.id} emails={person.emails} />
          <PhoneManager personId={person.id} phones={person.phones} />
          <CallSignManager personId={person.id} callSigns={person.callSigns} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Account &amp; login</T></h2>
          <p className="mt-1 text-xs text-slate-400">
            <T>Bind this person to an external IdP (Keycloak) login. issuer = realm URL (token `iss`); subject = the user&apos;s `sub` UUID.</T>
          </p>
          <AccountManager personId={person.id} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Social &amp; messenger</T></h2>
          <SocialAccountManager personId={person.id} accounts={person.socialAccounts} />
          <MessengerLinkManager
            personId={person.id}
            links={person.messengerLinks}
            emails={person.emails}
            phones={person.phones}
          />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Languages</T></h2>
          <PersonLanguageManager personId={person.id} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Citizenship &amp; residence</T></h2>
          <CitizenshipManager personId={person.id} citizenships={person.citizenships} />
          <ResidenceManager personId={person.id} residences={person.residences} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Name variants</T></h2>
          <NameVariantManager personId={person.id} variants={person.nameVariants} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Religion</T></h2>
          <PersonClergyManager personId={person.id} />
          <PersonAffiliationManager personId={person.id} />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Physical identity</T></h2>
          <PhysicalIdentityManager personId={person.id} />
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
  const ok = await oikumenea();
  const [documents, codes, memberships, orders, partnerships, kinships, guardianships, sponsorships, nextOfKin, associations] =
    await Promise.all([
      ok.document.listPersonDocuments(personId).catch(() => null),
      ok.document.listPersonPersonalCodes(personId).catch(() => null),
      ok.membership.listPersonMemberships(personId).catch(() => null),
      ok.order.listPersonOrders(personId).catch(() => null),
      ok.person.listPartnerships(personId).catch(() => []),
      ok.person.listKinships(personId).catch(() => []),
      ok.person.listGuardianships(personId).catch(() => []),
      ok.person.listSponsorships(personId).catch(() => []),
      ok.person.listNextOfKin(personId).catch(() => []),
      ok.person.listAssociations(personId).catch(() => []),
    ]);

  return (
    <>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900"><T>Relationships</T></h2>
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
          <h2 className="text-sm font-semibold text-slate-900"><T>Documents &amp; personal codes</T></h2>
          <DocumentManager personId={person.id} documents={documents?.documents} />
          <PersonalCodeManager personId={person.id} codes={codes ?? undefined} />
        </Card>

        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900"><T>Education</T></h2>
          <PersonEducationManager personId={person.id} />
        </Card>

        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900"><T>Companies</T></h2>
          <PersonCompaniesManager personId={person.id} />
        </Card>

        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900"><T>Vehicles</T></h2>
          <PersonVehiclesManager personId={person.id} />
        </Card>
      </div>

      <Section title={<T>Memberships</T>}>
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
          <EmptyState><T>No memberships.</T></EmptyState>
        )}
      </Section>

      <Section title={<T>Orders</T>}>
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
          <EmptyState><T>No orders reference this person.</T></EmptyState>
        )}
      </Section>
    </>
  );
}

function StreamFallback() {
  return <div className="mt-4 text-sm text-slate-400"><T>Loading relationships, documents, memberships…</T></div>;
}

function Section({ title, children }: { title: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="mt-8">
      <h2 className="mb-3 text-sm font-semibold text-slate-900">{title}</h2>
      {children}
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
