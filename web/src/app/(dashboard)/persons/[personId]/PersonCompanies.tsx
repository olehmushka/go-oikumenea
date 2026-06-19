"use client";

// PersonCompaniesManager — a read-only view of a person's company affiliations (M21 / D-Companies):
// employment appointments (enriched with company + title), foundings, shareholdings, and the companies
// they are a beneficial owner of. These links are recorded/edited from the Companies workspace; here we
// only surface them on the person object view (mirrors the read-only AppointmentSection of education).

import { useEffect, useState } from "react";
import { bffGet } from "@/lib/api/browser";
import { ErrorBox } from "@/components/ErrorBox";

type Appointment = { id: string; positionTitle: string; companyId: string; companyName: string; status: string; effectiveFrom?: string; effectiveTo?: string };
type Founding = { id: string; companyId: string; companyLabel?: string; foundedOn?: string };
type Shareholding = { id: string; companyId: string; companyLabel?: string; stakePct?: number };
type Beneficiary = { id: string; companyId: string; companyLabel?: string; ultimatePct?: number; declared: boolean };
type Affiliations = { appointments: Appointment[]; foundings: Founding[]; shareholdings: Shareholding[]; beneficiaryOf: Beneficiary[] };

const tail = (id: string) => id.slice(-8);
const pct = (v?: number) => (v == null ? "" : ` · ${v}%`);

export function PersonCompaniesManager({ personId }: { personId: string }) {
  const [aff, setAff] = useState<Affiliations | null>(null);
  const [err, setErr] = useState<unknown>(null);

  useEffect(() => {
    bffGet<Affiliations>(`/company/v1/persons/${personId}/company-affiliations`).then(setAff).catch(setErr);
  }, [personId]);

  if (err) return <ErrorBox error={err} />;
  const empty = aff && aff.appointments.length === 0 && aff.foundings.length === 0 && aff.shareholdings.length === 0 && aff.beneficiaryOf.length === 0;

  return (
    <div className="space-y-3 text-sm text-slate-700">
      {empty ? <p className="text-slate-400">No company affiliations.</p> : null}
      {aff && aff.appointments.length > 0 ? (
        <Group title="Employment">
          {aff.appointments.map((a) => <li key={a.id}>{a.positionTitle} · {a.companyName} · {a.status}{a.effectiveFrom ? ` · ${a.effectiveFrom}→${a.effectiveTo ?? "now"}` : ""}</li>)}
        </Group>
      ) : null}
      {aff && aff.foundings.length > 0 ? (
        <Group title="Founded">{aff.foundings.map((f) => <li key={f.id}>{f.companyLabel || tail(f.companyId)}{f.foundedOn ? ` · ${f.foundedOn}` : ""}</li>)}</Group>
      ) : null}
      {aff && aff.shareholdings.length > 0 ? (
        <Group title="Shareholdings">{aff.shareholdings.map((s) => <li key={s.id}>{s.companyLabel || tail(s.companyId)}{pct(s.stakePct)}</li>)}</Group>
      ) : null}
      {aff && aff.beneficiaryOf.length > 0 ? (
        <Group title="Beneficial owner of">{aff.beneficiaryOf.map((b) => <li key={b.id}>{b.companyLabel || tail(b.companyId)}{pct(b.ultimatePct)}</li>)}</Group>
      ) : null}
      <p className="mt-1 text-xs text-slate-400">Read-only. Company links are recorded from the Companies workspace.</p>
    </div>
  );
}

function Group({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-400">{title}</p>
      <ul className="space-y-0.5">{children}</ul>
    </div>
  );
}
