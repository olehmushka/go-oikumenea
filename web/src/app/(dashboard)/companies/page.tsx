"use client";

// Company workspace (M21 / D-Companies). Browse/create legal entities and drill into one to see its
// registrations, industries, locations, positions, and the ownership/affiliation graph (shareholders,
// holdings/subsidiaries, beneficial owners, founders, successions, branches). Per-object views live at
// /o/<id>; a person's company affiliations are managed from the person object view.

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { mutate } from "@/lib/api/client";
import { bffGet } from "@/lib/api/browser";
import { CountrySelect } from "@/components/CountrySelect";
import { SearchSelect, type SearchKind } from "@/components/SearchSelect";
import { PersonLink } from "@/components/PositionForms";
import { PageHeader, Card, Table, Mono } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { newSuffix, slugify } from "@/lib/code";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Catalog = { id: string; code: string; name: LocaleMap };
type Company = { id: string; code: string; legalName: LocaleMap; shortName?: string; ownershipCategory: string; state: string };
type Registration = { id: string; schemeId: string; identifier: string; validated: boolean };
type IndustryAssignment = { id: string; industryClassId: string; isPrimary: boolean };
type CompanyLocation = { id: string; locationId: string; role: string };
type Position = { id: string; code: string; title: LocaleMap; status: string; holder?: { personId: string } };
type Founding = { id: string; holderKind: string; holderId: string; holderLabel?: string; foundedOn?: string };
type Shareholding = { id: string; companyId: string; companyLabel?: string; holderKind: string; holderId: string; holderLabel?: string; stakePct?: number };
type Beneficiary = { id: string; personId: string; ultimatePct?: number; declared: boolean };
type Succession = { id: string; predecessorId: string; predecessorLabel?: string; successorId: string; successorLabel?: string; kind: string };
type Branch = { id: string; branchId: string; branchLabel?: string; parentId: string };
type Graph = {
  shareholders: Shareholding[]; holdings: Shareholding[]; beneficiaries: Beneficiary[];
  founders: Founding[]; successions: Succession[]; branches: Branch[];
};

const C = "/company/v1";
const OWNERSHIP = ["private", "public", "state_owned", "municipal", "foreign", "mixed"];

export default function CompaniesPage() {
  const [legalForms, setLegalForms] = useState<Catalog[]>([]);
  const [companies, setCompanies] = useState<Company[]>([]);
  const [selected, setSelected] = useState<Company | null>(null);
  const [err, setErr] = useState<unknown>(null);

  function reload() {
    bffGet<{ companies: Company[] }>(`${C}/companies?pageSize=100`).then((r) => setCompanies(r.companies ?? [])).catch(setErr);
  }
  useEffect(() => {
    bffGet<{ legalForms: Catalog[] }>(`${C}/legal-forms`).then((r) => setLegalForms(r.legalForms ?? [])).catch(() => {});
    reload();
  }, []);

  return (
    <div>
      <PageHeader
        title="Companies"
        description="A legal-entity registry — identity, legal form, registrations, locations, positions, and the ownership/affiliation graph. External reference data, independent of the deploying org's units."
      />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <CreateCompany legalForms={legalForms} onCreated={reload} />
        <CompanyList companies={companies} selected={selected} onSelect={setSelected} />
      </div>
      {selected ? (
        <div className="mt-6 space-y-6">
          <CompanyDetail company={selected} />
          <OwnershipPanel company={selected} companies={companies} />
        </div>
      ) : null}
    </div>
  );
}

function CreateCompany({ legalForms, onCreated }: { legalForms: Catalog[]; onCreated: () => void }) {
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<Company | null>(null);
  const suffix = useRef(newSuffix());
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  const slug = slugify(name);
  const codeValue = codeTouched ? code : slug ? `${slug}-${suffix.current}` : "";

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true); setErr(null); setCreated(null);
    const f = new FormData(e.currentTarget);
    const str = (k: string) => { const v = String(f.get(k) || "").trim(); return v === "" ? undefined : v; };
    try {
      const c = await mutate<Company>("POST", `${C}/companies`, {
        code: codeValue.trim(), legalName: name.trim(), shortName: str("shortName"),
        legalFormId: String(f.get("legalFormId") || "").trim(),
        ownershipCategory: str("ownershipCategory"), countryId: str("countryId"), foundedOn: str("foundedOn"),
      });
      setCreated(c);
      (e.target as HTMLFormElement).reset();
      setName(""); setCode(""); setCodeTouched(false); suffix.current = newSuffix();
      onCreated();
    } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">Register a company</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <div><label className="label">Legal name *</label><input required className="input" placeholder="Acme LLC" value={name} onChange={(e) => setName(e.target.value)} /></div>
          <div><label className="label">Code *</label><input name="code" required className="input" placeholder="auto from name" value={codeValue} onChange={(e) => { setCode(e.target.value); setCodeTouched(true); }} /></div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div><label className="label">Short name</label><input name="shortName" className="input" placeholder="Acme" /></div>
          <div>
            <label className="label">Legal form *</label>
            <select name="legalFormId" required className="input" defaultValue="">
              <option value="" disabled>—</option>
              {legalForms.map((k) => <option key={k.id} value={k.id}>{pickLabel(k.name) || k.code}</option>)}
            </select>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="label">Ownership</label>
            <select name="ownershipCategory" className="input" defaultValue="private">
              {OWNERSHIP.map((o) => <option key={o} value={o}>{o}</option>)}
            </select>
          </div>
          <div><label className="label">Country</label><CountrySelect name="countryId" /></div>
        </div>
        <div><label className="label">Founded</label><input name="foundedOn" type="date" className="input" /></div>
        <button type="submit" className="btn" disabled={busy}>{busy ? "Creating…" : "Register company"}</button>
      </form>
      {created ? (
        <div className="mt-4 rounded-md border border-green-200 bg-green-50 p-3 text-sm text-green-800">
          Created <Mono>{created.code}</Mono> — <Link href={`/o/${created.id}`} className="underline">open</Link>
        </div>
      ) : null}
    </Card>
  );
}

function CompanyList({ companies, selected, onSelect }: { companies: Company[]; selected: Company | null; onSelect: (c: Company) => void }) {
  const [q, setQ] = useState("");
  const shown = companies.filter((c) => {
    if (!q.trim()) return true;
    const t = q.toLowerCase();
    return c.code.toLowerCase().includes(t) || (pickLabel(c.legalName) || "").toLowerCase().includes(t);
  });
  return (
    <Card>
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold text-slate-700">Companies</h2>
        <input className="input w-48" placeholder="filter…" value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      {shown.length === 0 ? (
        <p className="text-sm text-slate-400">No companies.</p>
      ) : (
        <Table head={<><th className="th">Code</th><th className="th">Legal name</th><th className="th">Ownership</th><th className="th">State</th></>}>
          {shown.map((c) => (
            <tr key={c.id} className={`cursor-pointer hover:bg-slate-50 ${selected?.id === c.id ? "bg-indigo-50" : ""}`} onClick={() => onSelect(c)}>
              <td className="td"><Mono>{c.code}</Mono></td>
              <td className="td">{pickLabel(c.legalName) || "—"}</td>
              <td className="td">{c.ownershipCategory}</td>
              <td className="td">{c.state}</td>
            </tr>
          ))}
        </Table>
      )}
    </Card>
  );
}

function CompanyDetail({ company }: { company: Company }) {
  const [schemes, setSchemes] = useState<Catalog[]>([]);
  const [industries, setIndustries] = useState<Catalog[]>([]);
  const [regs, setRegs] = useState<Registration[]>([]);
  const [inds, setInds] = useState<IndustryAssignment[]>([]);
  const [locs, setLocs] = useState<CompanyLocation[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  // Remounts the location form (controlled SearchSelect) after a successful add — form.reset() can't
  // clear the picker's React state.
  const [nonce, setNonce] = useState(0);

  function reload() {
    bffGet<{ registrations: Registration[] }>(`${C}/companies/${company.id}/registrations`).then((r) => setRegs(r.registrations ?? [])).catch(setErr);
    bffGet<{ industries: IndustryAssignment[] }>(`${C}/companies/${company.id}/industries`).then((r) => setInds(r.industries ?? [])).catch(() => {});
    bffGet<{ locations: CompanyLocation[] }>(`${C}/companies/${company.id}/locations`).then((r) => setLocs(r.locations ?? [])).catch(() => {});
    bffGet<{ positions: Position[] }>(`${C}/companies/${company.id}/positions`).then((r) => setPositions(r.positions ?? [])).catch(() => {});
  }
  useEffect(() => {
    bffGet<{ registrationSchemes: Catalog[] }>(`${C}/registration-schemes`).then((r) => setSchemes(r.registrationSchemes ?? [])).catch(() => {});
    bffGet<{ industryClasses: Catalog[] }>(`${C}/industry-classes`).then((r) => setIndustries(r.industryClasses ?? [])).catch(() => {});
  }, []);
  useEffect(reload, [company.id]);

  const schemeName = (id: string) => pickLabel(schemes.find((s) => s.id === id)?.name) || schemes.find((s) => s.id === id)?.code || id.slice(-6);
  const industryName = (id: string) => pickLabel(industries.find((s) => s.id === id)?.name) || industries.find((s) => s.id === id)?.code || id.slice(-6);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true); setErr(null);
    try { await fn(); reload(); setNonce((n) => n + 1); } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  async function addReg(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    await run(() => mutate("POST", `${C}/companies/${company.id}/registrations`, {
      schemeId: String(f.get("schemeId") || ""), identifier: String(f.get("identifier") || "").trim(),
    }));
    (e.target as HTMLFormElement).reset();
  }
  async function addIndustry(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    await run(() => mutate("POST", `${C}/companies/${company.id}/industries`, {
      industryClassId: String(f.get("industryClassId") || ""), isPrimary: f.get("isPrimary") === "on",
    }));
    (e.target as HTMLFormElement).reset();
  }
  async function addLocation(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const locationId = String(f.get("locationId") || "").trim();
    if (!locationId) return;
    await run(() => mutate("POST", `${C}/companies/${company.id}/locations`, {
      locationId, role: String(f.get("role") || "registered"),
    }));
    (e.target as HTMLFormElement).reset();
  }
  async function addPosition(code: string, title: string) {
    await run(() => mutate("POST", `${C}/companies/${company.id}/positions`, { code, title }));
  }
  async function fill(positionId: string, personId: string) {
    if (!personId) return;
    await run(() => mutate("POST", `${C}/positions/${positionId}/fill`, { personId }));
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">{pickLabel(company.legalName) || company.code} — registry</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase text-slate-500">Registrations</h3>
          {regs.length === 0 ? <p className="text-sm text-slate-400">None.</p> : (
            <ul className="space-y-1 text-sm">
              {regs.map((r) => (
                <li key={r.id} className="flex items-center justify-between">
                  <span><Mono>{schemeName(r.schemeId)}</Mono> {r.identifier} {r.validated ? <span className="text-green-600">✓</span> : <span className="text-amber-600">unvalidated</span>}</span>
                  <button className="text-xs text-red-600 hover:underline" disabled={busy} onClick={() => run(() => mutate("DELETE", `${C}/registrations/${r.id}`))}>remove</button>
                </li>
              ))}
            </ul>
          )}
          <form onSubmit={addReg} className="mt-3 grid grid-cols-[1fr_1fr_auto] gap-2">
            <select name="schemeId" required className="input"><option value="" disabled>scheme…</option>{schemes.map((s) => <option key={s.id} value={s.id}>{pickLabel(s.name) || s.code}</option>)}</select>
            <input name="identifier" required className="input" placeholder="identifier" />
            <button className="btn" disabled={busy}>Add</button>
          </form>

          <h3 className="mb-2 mt-5 text-xs font-semibold uppercase text-slate-500">Industries</h3>
          {inds.length === 0 ? <p className="text-sm text-slate-400">None.</p> : (
            <ul className="space-y-1 text-sm">
              {inds.map((a) => (
                <li key={a.id} className="flex items-center justify-between">
                  <span>{industryName(a.industryClassId)}{a.isPrimary ? <span className="ml-1 text-indigo-600">(primary)</span> : ""}</span>
                  <button className="text-xs text-red-600 hover:underline" disabled={busy} onClick={() => run(() => mutate("DELETE", `${C}/industries/${a.id}`))}>remove</button>
                </li>
              ))}
            </ul>
          )}
          <form onSubmit={addIndustry} className="mt-3 grid grid-cols-[1fr_auto_auto] items-center gap-2">
            <select name="industryClassId" required className="input"><option value="" disabled>class…</option>{industries.map((s) => <option key={s.id} value={s.id}>{pickLabel(s.name) || s.code}</option>)}</select>
            <label className="flex items-center gap-1 text-xs text-slate-600"><input type="checkbox" name="isPrimary" /> primary</label>
            <button className="btn" disabled={busy}>Add</button>
          </form>

          <h3 className="mb-2 mt-5 text-xs font-semibold uppercase text-slate-500">Locations</h3>
          {locs.length === 0 ? <p className="text-sm text-slate-400">None.</p> : (
            <ul className="space-y-1 text-sm">
              {locs.map((l) => (
                <li key={l.id} className="flex items-center justify-between">
                  <span><LocationLabel id={l.locationId} /> · {l.role}</span>
                  <button className="text-xs text-red-600 hover:underline" disabled={busy} onClick={() => run(() => mutate("DELETE", `${C}/company-locations/${l.id}`))}>remove</button>
                </li>
              ))}
            </ul>
          )}
          <form key={`loc-${nonce}`} onSubmit={addLocation} className="mt-3 grid grid-cols-[1fr_auto_auto] gap-2">
            <SearchSelect kind="location" name="locationId" required placeholder="Search a location…" />
            <select name="role" className="input">{["registered", "operating", "branch"].map((r) => <option key={r} value={r}>{r}</option>)}</select>
            <button className="btn" disabled={busy}>Add</button>
          </form>
        </div>
        <PositionsSection positions={positions} busy={busy} onFill={fill} onAdd={addPosition} />
      </div>
    </Card>
  );
}

// Resolves a company-location's RID to a human label (locality / MGRS / coordinate), caching per id so
// repeated rows don't refetch — avoids showing a raw RID tail in the list (D-WebUI UX).
const locationLabelCache = new Map<string, string>();
function LocationLabel({ id }: { id: string }) {
  const [label, setLabel] = useState<string>(() => locationLabelCache.get(id) ?? "");
  useEffect(() => {
    if (locationLabelCache.has(id)) { setLabel(locationLabelCache.get(id) ?? ""); return; }
    let alive = true;
    bffGet<{ locality?: string; mgrs?: string; latitude?: number; longitude?: number }>(`/location/v1/locations/${id}`)
      .then((l) => {
        const coords = l.latitude != null && l.longitude != null ? `${l.latitude.toFixed(4)}, ${l.longitude.toFixed(4)}` : "";
        const text = l.locality || l.mgrs || coords || "";
        locationLabelCache.set(id, text);
        if (alive) setLabel(text);
      })
      .catch(() => {});
    return () => { alive = false; };
  }, [id]);
  return label ? <span className="text-slate-700">{label}</span> : <Mono>{id.slice(-8)}</Mono>;
}

// Positions sub-panel: the table lives in an overflow-hidden card, so the appoint-a-person typeahead
// is rendered in a panel BELOW the table (where its dropdown can't be clipped). Creating a position
// auto-fills the code from the title (live-fill), matching CreateCompany and the unit PositionForms.
function PositionsSection({
  positions, busy, onFill, onAdd,
}: {
  positions: Position[];
  busy: boolean;
  onFill: (positionId: string, personId: string) => void;
  onAdd: (code: string, title: string) => void;
}) {
  const [fillingId, setFillingId] = useState<string | null>(null);
  const [personId, setPersonId] = useState("");
  const filling = positions.find((p) => p.id === fillingId) || null;

  const suffix = useRef(newSuffix());
  const [title, setTitle] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  const slug = slugify(title);
  const codeValue = codeTouched ? code : slug ? `${slug}-${suffix.current}` : "";

  function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!title.trim() || !codeValue.trim()) return;
    onAdd(codeValue.trim(), title.trim());
    setTitle(""); setCode(""); setCodeTouched(false); suffix.current = newSuffix();
  }

  return (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase text-slate-500">Positions</h3>
      {positions.length === 0 ? <p className="text-sm text-slate-400">None.</p> : (
        <Table head={<><th className="th">Title</th><th className="th">Holder</th><th className="th"></th></>}>
          {positions.map((p) => (
            <tr key={p.id} className="hover:bg-slate-50">
              <td className="td"><Link href={`/o/${p.id}`} className="text-indigo-600 hover:underline">{pickLabel(p.title) || p.code}</Link></td>
              <td className="td">{p.holder ? <PersonLink personId={p.holder.personId} /> : <span className="text-amber-600">vacant</span>}</td>
              <td className="td">{p.holder ? null : <button className="text-xs text-indigo-600 hover:underline" disabled={busy} onClick={() => { setFillingId(p.id); setPersonId(""); }}>fill</button>}</td>
            </tr>
          ))}
        </Table>
      )}
      {filling ? (
        <div className="mt-3 rounded-md border border-indigo-200 bg-indigo-50/50 p-3">
          <div className="mb-2 text-xs text-slate-600">Appoint to <span className="font-semibold">{pickLabel(filling.title) || filling.code}</span></div>
          <div className="flex items-center gap-2">
            <div className="flex-1"><SearchSelect kind="person" onChange={setPersonId} placeholder="Search a person…" /></div>
            <button className="btn disabled:opacity-40" disabled={busy || !personId} onClick={() => { onFill(filling.id, personId); setFillingId(null); setPersonId(""); }}>Appoint</button>
            <button className="text-xs text-slate-500 hover:underline" onClick={() => { setFillingId(null); setPersonId(""); }}>cancel</button>
          </div>
        </div>
      ) : null}
      <form onSubmit={submit} className="mt-3 grid grid-cols-[1fr_1fr_auto] gap-2">
        <input className="input" placeholder="title (CEO)" value={title} onChange={(e) => setTitle(e.target.value)} />
        <input className="input" placeholder="auto from title" value={codeValue} onChange={(e) => { setCode(e.target.value); setCodeTouched(true); }} />
        <button type="submit" className="btn" disabled={busy}>Add</button>
      </form>
    </div>
  );
}

function OwnershipPanel({ company, companies }: { company: Company; companies: Company[] }) {
  const [g, setG] = useState<Graph | null>(null);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  // Bumped after each successful mutation to remount the picker-bearing forms (controlled SearchSelect
  // state isn't cleared by a native form.reset()).
  const [nonce, setNonce] = useState(0);

  function reload() {
    bffGet<Graph & { companyId: string }>(`${C}/companies/${company.id}/ownership-graph`).then(setG).catch(setErr);
  }
  useEffect(reload, [company.id]);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true); setErr(null);
    try { await fn(); reload(); setNonce((n) => n + 1); } catch (e) { setErr(e); } finally { setBusy(false); }
  }
  async function addShareholding(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const holderId = String(f.get("holderId") || "").trim();
    if (!holderId) return;
    const pct = String(f.get("stakePct") || "").trim();
    await run(() => mutate("POST", `${C}/companies/${company.id}/shareholdings`, {
      holderKind: String(f.get("holderKind") || "company"), holderId,
      stakePct: pct === "" ? undefined : Number(pct),
    }));
    (e.target as HTMLFormElement).reset();
  }
  async function addFounding(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const holderId = String(f.get("holderId") || "").trim();
    if (!holderId) return;
    await run(() => mutate("POST", `${C}/companies/${company.id}/foundings`, {
      holderKind: String(f.get("holderKind") || "person"), holderId,
    }));
    (e.target as HTMLFormElement).reset();
  }
  async function addBeneficiary(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const personId = String(f.get("personId") || "").trim();
    if (!personId) return;
    const pct = String(f.get("ultimatePct") || "").trim();
    await run(() => mutate("POST", `${C}/companies/${company.id}/beneficiaries`, {
      personId, ultimatePct: pct === "" ? undefined : Number(pct),
    }));
    (e.target as HTMLFormElement).reset();
  }
  async function addBranch(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    await run(() => mutate("POST", `${C}/companies/${company.id}/branches`, { branchId: String(f.get("branchId") || "") }));
  }
  async function addSuccession(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    await run(() => mutate("POST", `${C}/companies/${company.id}/successions`, {
      successorId: String(f.get("successorId") || ""), kind: String(f.get("kind") || "reorganization"),
    }));
  }

  const others = companies.filter((c) => c.id !== company.id);
  const pct = (v?: number) => (v == null ? "" : ` · ${v}%`);

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">Ownership &amp; affiliation graph</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div className="space-y-4">
          <GraphList title="Shareholders (stakes held IN this company)" rows={(g?.shareholders ?? []).map((s) => `${s.holderKind}: ${s.holderLabel || s.holderId.slice(-8)}${pct(s.stakePct)}`)} />
          <form key={`sh-${nonce}`} onSubmit={addShareholding} className="grid grid-cols-[auto_1fr_auto_auto] items-center gap-2">
            <HolderPicker defaultKind="company" />
            <input name="stakePct" type="number" step="0.01" className="input w-20" placeholder="%" />
            <button className="btn" disabled={busy}>Add</button>
          </form>

          <GraphList title="Holdings (subsidiaries — stakes this company holds)" rows={(g?.holdings ?? []).map((s) => `${s.companyLabel || s.companyId.slice(-8)}${pct(s.stakePct)}`)} />
          <GraphList title="Founders" rows={(g?.founders ?? []).map((s) => `${s.holderKind}: ${s.holderLabel || s.holderId.slice(-8)}`)} />
          <form key={`fd-${nonce}`} onSubmit={addFounding} className="grid grid-cols-[auto_1fr_auto] items-center gap-2">
            <HolderPicker defaultKind="person" />
            <button className="btn" disabled={busy}>Add</button>
          </form>
        </div>
        <div className="space-y-4">
          <div>
            <h3 className="mb-1 text-xs font-semibold uppercase text-slate-500">Beneficial owners (UBO)</h3>
            {(g?.beneficiaries ?? []).length === 0 ? <p className="text-sm text-slate-400">None.</p> : (
              <ul className="space-y-0.5 text-sm text-slate-700">
                {(g?.beneficiaries ?? []).map((b) => (
                  <li key={b.id} className="flex items-center gap-1"><PersonLink personId={b.personId} /><span>{pct(b.ultimatePct)}{b.declared ? " · declared" : " · computed"}</span></li>
                ))}
              </ul>
            )}
          </div>
          <form key={`bn-${nonce}`} onSubmit={addBeneficiary} className="grid grid-cols-[1fr_auto_auto] items-center gap-2">
            <SearchSelect kind="person" name="personId" required placeholder="Search a person…" />
            <input name="ultimatePct" type="number" step="0.01" className="input w-20" placeholder="%" />
            <button className="btn" disabled={busy}>Add</button>
          </form>

          <GraphList title="Branches" rows={(g?.branches ?? []).map((b) => b.branchLabel || b.branchId.slice(-8))} />
          <form onSubmit={addBranch} className="grid grid-cols-[1fr_auto] gap-2">
            <select name="branchId" required className="input"><option value="" disabled>branch company…</option>{others.map((c) => <option key={c.id} value={c.id}>{pickLabel(c.legalName) || c.code}</option>)}</select>
            <button className="btn" disabled={busy}>Add</button>
          </form>

          <GraphList title="Successions" rows={(g?.successions ?? []).map((s) => `${s.predecessorLabel || s.predecessorId.slice(-8)} → ${s.successorLabel || s.successorId.slice(-8)} (${s.kind})`)} />
          <form onSubmit={addSuccession} className="grid grid-cols-[1fr_auto_auto] gap-2">
            <select name="successorId" required className="input"><option value="" disabled>successor…</option>{others.map((c) => <option key={c.id} value={c.id}>{pickLabel(c.legalName) || c.code}</option>)}</select>
            <select name="kind" className="input">{["merger", "reorganization", "rename", "acquisition", "spinoff"].map((k) => <option key={k} value={k}>{k}</option>)}</select>
            <button className="btn" disabled={busy}>Add</button>
          </form>
        </div>
      </div>
    </Card>
  );
}

// Holder picker for shareholding/founding: a kind toggle (company|person) + a server-query SearchSelect
// re-keyed on kind change (a company selection is invalid as a person, so reset). Both submit via hidden
// inputs (holderKind, holderId), so the FormData submit handlers stay unchanged.
function HolderPicker({ defaultKind }: { defaultKind: "company" | "person" }) {
  const [hk, setHk] = useState<"company" | "person">(defaultKind);
  return (
    <>
      <select name="holderKind" className="input" value={hk} onChange={(e) => setHk(e.target.value as "company" | "person")}>
        <option value="company">company</option>
        <option value="person">person</option>
      </select>
      <SearchSelect key={hk} kind={hk as SearchKind} name="holderId" required placeholder={`Search a ${hk}…`} />
    </>
  );
}

function GraphList({ title, rows }: { title: string; rows: string[] }) {
  return (
    <div>
      <h3 className="mb-1 text-xs font-semibold uppercase text-slate-500">{title}</h3>
      {rows.length === 0 ? <p className="text-sm text-slate-400">None.</p> : (
        <ul className="space-y-0.5 text-sm text-slate-700">{rows.map((r, i) => <li key={i}>{r}</li>)}</ul>
      )}
    </div>
  );
}
