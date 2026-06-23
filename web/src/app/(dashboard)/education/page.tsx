"use client";

// Education workspace (M20 / D-Education). Browse/create external reference institutions and drill into
// one to see its structure tree (units, with closure depth) + positions, and add a unit. Per-object
// views live at /o/<id>; person enrollments/dorm stays are managed from the person object view.

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { mutate } from "@/lib/api/client";
import { bffGet } from "@/lib/api/browser";
import { CountrySelect } from "@/components/CountrySelect";
import { PageHeader, Card, Table, Mono } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { newSuffix, slugify } from "@/lib/code";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Kind = { id: string; code: string; name: LocaleMap };
type Institution = { id: string; code: string; name: LocaleMap; state: string };
type Unit = { id: string; code: string; name: LocaleMap; parentId?: string; status: string; depth?: number };
type Position = { id: string; code: string; title: LocaleMap; status: string; holder?: { personId: string } };

export default function EducationPage() {
  const [institutionKinds, setInstitutionKinds] = useState<Kind[]>([]);
  const [unitKinds, setUnitKinds] = useState<Kind[]>([]);
  const [institutions, setInstitutions] = useState<Institution[]>([]);
  const [selected, setSelected] = useState<Institution | null>(null);
  const [err, setErr] = useState<unknown>(null);

  function reload() {
    bffGet<{ institutions: Institution[] }>("/education/v1/institutions?pageSize=100")
      .then((r) => setInstitutions(r.institutions ?? []))
      .catch(setErr);
  }

  useEffect(() => {
    bffGet<{ institutionKinds: Kind[] }>("/education/v1/institution-kinds").then((r) => setInstitutionKinds(r.institutionKinds ?? [])).catch(() => {});
    bffGet<{ unitKinds: Kind[] }>("/education/v1/unit-kinds").then((r) => setUnitKinds(r.unitKinds ?? [])).catch(() => {});
    reload();
  }, []);

  return (
    <div>
      <PageHeader
        title={<T>Education</T>}
        description={<T>External reference institutions (where people studied/taught) and their internal structure tree. Distinct from the deploying org's tenant units.</T>}
      />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <CreateInstitution kinds={institutionKinds} onCreated={reload} />
        <InstitutionList institutions={institutions} selected={selected} onSelect={setSelected} />
      </div>
      {selected ? (
        <div className="mt-6 space-y-6">
          <InstitutionDetail institution={selected} unitKinds={unitKinds} />
          <ReferencePanel institution={selected} />
        </div>
      ) : null}
    </div>
  );
}

// ReferencePanel: create + browse the institution-scoped reference entities (M20 extension). Each entity
// shares a create shape (code + name/title); the selected type drives the endpoint + list key.
type RefRow = { id: string; code: string; name?: string; title?: string };
const REF_TYPES = [
  { key: "programs", label: "Programs", nameField: "name", listKey: "programs" },
  { key: "courses", label: "Courses", nameField: "title", listKey: "courses" },
  { key: "research-centres", label: "Research centres", nameField: "name", listKey: "researchCentres" },
  { key: "grants", label: "Grants", nameField: "title", listKey: "grants" },
  { key: "governance-bodies", label: "Governance bodies", nameField: "name", listKey: "governanceBodies" },
  { key: "qualifications", label: "Qualifications", nameField: "name", listKey: "qualifications" },
] as const;

function ReferencePanel({ institution }: { institution: Institution }) {
  const tr = useTg();
  const [kind, setKind] = useState<(typeof REF_TYPES)[number]>(REF_TYPES[0]);
  const [rows, setRows] = useState<RefRow[]>([]);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const suffix = useRef(newSuffix());
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  const slug = slugify(name);
  const codeValue = codeTouched ? code : slug ? `${slug}-${suffix.current}` : "";

  function reload() {
    bffGet<Record<string, RefRow[]>>(`/education/v1/institutions/${institution.id}/${kind.key}`)
      .then((r) => setRows(r[kind.listKey] ?? []))
      .catch(setErr);
  }
  useEffect(reload, [institution.id, kind.key]);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      const body: Record<string, string> = { code: codeValue.trim() };
      body[kind.nameField] = name.trim();
      await mutate("POST", `/education/v1/institutions/${institution.id}/${kind.key}`, body);
      setName(""); setCode(""); setCodeTouched(false); suffix.current = newSuffix();
      reload();
    } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  return (
    <Card>
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-slate-700"><T>Reference layer</T></h2>
        <select className="input w-48" value={kind.key} onChange={(e) => setKind(REF_TYPES.find((t) => t.key === e.target.value) ?? REF_TYPES[0])}>
          {REF_TYPES.map((t) => <option key={t.key} value={t.key}>{tr(t.label)}</option>)}
        </select>
      </div>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="mb-4 grid grid-cols-1 gap-2 sm:grid-cols-[1fr_1fr_auto]">
        <input required className="input" placeholder={tr(kind.nameField === "title" ? "title" : "name")} value={name} onChange={(e) => setName(e.target.value)} />
        <input required className="input" placeholder={tr("auto from name")} value={codeValue} onChange={(e) => { setCode(e.target.value); setCodeTouched(true); }} />
        <button type="submit" className="btn" disabled={busy}>{busy ? <T>Adding…</T> : <T>Add</T>}</button>
      </form>
      {rows.length === 0 ? <p className="text-sm text-slate-400"><T>None yet.</T></p> : (
        <Table head={<><th className="th"><T>Code</T></th><th className="th"><T>Name</T></th></>}>
          {rows.map((r) => (
            <tr key={r.id} className="hover:bg-slate-50">
              <td className="td"><Mono>{r.code}</Mono></td>
              <td className="td"><Link href={`/o/${r.id}`} className="text-indigo-600 hover:underline">{r.name ?? r.title ?? r.code}</Link></td>
            </tr>
          ))}
        </Table>
      )}
    </Card>
  );
}

function CreateInstitution({ kinds, onCreated }: { kinds: Kind[]; onCreated: () => void }) {
  const tr = useTg();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<Institution | null>(null);

  // Live-fill the code from the name until the operator edits it (stable per-form suffix).
  const suffix = useRef(newSuffix());
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  const slug = slugify(name);
  const codeValue = codeTouched ? code : slug ? `${slug}-${suffix.current}` : "";

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    setCreated(null);
    const f = new FormData(e.currentTarget);
    const str = (k: string) => { const v = String(f.get(k) || "").trim(); return v === "" ? undefined : v; };
    try {
      const inst = await mutate<Institution>("POST", "/education/v1/institutions", {
        code: codeValue.trim(),
        name: name.trim(),
        kindId: String(f.get("kindId") || "").trim(),
        countryId: str("countryId"),
        foundedOn: str("foundedOn"),
      });
      setCreated(inst);
      (e.target as HTMLFormElement).reset();
      setName("");
      setCode("");
      setCodeTouched(false);
      suffix.current = newSuffix();
      onCreated();
    } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Create an institution</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="space-y-3">
        <div className="grid grid-cols-2 gap-3">
          <div><label className="label"><T>Name *</T></label><input name="name" required className="input" placeholder={tr("KPI")} value={name} onChange={(e) => setName(e.target.value)} /></div>
          <div><label className="label"><T>Code *</T></label><input name="code" required className="input" placeholder={tr("auto from name")} value={codeValue} onChange={(e) => { setCode(e.target.value); setCodeTouched(true); }} /></div>
        </div>
        <div>
          <label className="label"><T>Kind *</T></label>
          <select name="kindId" required className="input" defaultValue="">
            <option value="" disabled>—</option>
            {kinds.map((k) => <option key={k.id} value={k.id}>{pickLabel(k.name) || k.code}</option>)}
          </select>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div><label className="label"><T>Country</T></label><CountrySelect name="countryId" /></div>
          <div><label className="label"><T>Founded</T></label><input name="foundedOn" type="date" className="input" /></div>
        </div>
        <button type="submit" className="btn" disabled={busy}>{busy ? <T>Creating…</T> : <T>Create institution</T>}</button>
      </form>
      {created ? (
        <div className="mt-4 rounded-md border border-green-200 bg-green-50 p-3 text-sm text-green-800">
          <T>Created</T> <Mono>{created.code}</Mono> — <Link href={`/o/${created.id}`} className="underline"><T>open</T></Link>
        </div>
      ) : null}
    </Card>
  );
}

function InstitutionList({ institutions, selected, onSelect }: { institutions: Institution[]; selected: Institution | null; onSelect: (i: Institution) => void }) {
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Institutions</T></h2>
      {institutions.length === 0 ? (
        <p className="text-sm text-slate-400"><T>No institutions yet.</T></p>
      ) : (
        <Table head={<><th className="th"><T>Code</T></th><th className="th"><T>Name</T></th><th className="th"><T>State</T></th></>}>
          {institutions.map((i) => (
            <tr key={i.id} className={`cursor-pointer hover:bg-slate-50 ${selected?.id === i.id ? "bg-indigo-50" : ""}`} onClick={() => onSelect(i)}>
              <td className="td"><Mono>{i.code}</Mono></td>
              <td className="td">{pickLabel(i.name) || "—"}</td>
              <td className="td">{i.state}</td>
            </tr>
          ))}
        </Table>
      )}
    </Card>
  );
}

function InstitutionDetail({ institution, unitKinds }: { institution: Institution; unitKinds: Kind[] }) {
  const tr = useTg();
  const [units, setUnits] = useState<Unit[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  // Live-fill the code from the name until the operator edits it (stable per-form suffix).
  const suffix = useRef(newSuffix());
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  const slug = slugify(name);
  const codeValue = codeTouched ? code : slug ? `${slug}-${suffix.current}` : "";

  function reload() {
    bffGet<{ units: Unit[] }>(`/education/v1/institutions/${institution.id}/units`).then((r) => setUnits(r.units ?? [])).catch(setErr);
    bffGet<{ positions: Position[] }>(`/education/v1/institutions/${institution.id}/positions`).then((r) => setPositions(r.positions ?? [])).catch(() => {});
  }
  useEffect(reload, [institution.id]);

  async function addUnit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    const f = new FormData(e.currentTarget);
    const parent = String(f.get("parentId") || "").trim();
    try {
      await mutate("POST", `/education/v1/institutions/${institution.id}/units`, {
        code: codeValue.trim(),
        name: name.trim(),
        kindId: String(f.get("kindId") || "").trim(),
        parentId: parent === "" ? undefined : parent,
      });
      (e.target as HTMLFormElement).reset();
      setName("");
      setCode("");
      setCodeTouched(false);
      suffix.current = newSuffix();
      reload();
    } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">
        {pickLabel(institution.name) || institution.code} <T>— structure</T>
      </h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase text-slate-500"><T>Units (tree)</T></h3>
          {units.length === 0 ? <p className="text-sm text-slate-400"><T>No units.</T></p> : (
            <ul className="space-y-1 text-sm">
              {units.map((u) => (
                <li key={u.id} style={{ paddingLeft: `${(u.depth ?? 0) * 16}px` }}>
                  <Link href={`/o/${u.id}`} className="text-indigo-600 hover:underline">{pickLabel(u.name) || u.code}</Link>
                  <span className="ml-2 text-xs text-slate-400">{u.code}</span>
                </li>
              ))}
            </ul>
          )}
          <form onSubmit={addUnit} className="mt-4 space-y-2 border-t border-slate-100 pt-3">
            <div className="grid grid-cols-2 gap-2">
              <input name="name" required className="input" placeholder={tr("name (FIOT)")} value={name} onChange={(e) => setName(e.target.value)} />
              <input name="code" required className="input" placeholder={tr("auto from name")} value={codeValue} onChange={(e) => { setCode(e.target.value); setCodeTouched(true); }} />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <select name="kindId" required className="input" defaultValue="">
                <option value="" disabled>{tr("kind…")}</option>
                {unitKinds.map((k) => <option key={k.id} value={k.id}>{pickLabel(k.name) || k.code}</option>)}
              </select>
              <select name="parentId" className="input" defaultValue="">
                <option value="">{tr("— top-level —")}</option>
                {units.map((u) => <option key={u.id} value={u.id}>{pickLabel(u.name) || u.code}</option>)}
              </select>
            </div>
            <button type="submit" className="btn" disabled={busy}>{busy ? <T>Adding…</T> : <T>Add unit</T>}</button>
          </form>
        </div>
        <div>
          <h3 className="mb-2 text-xs font-semibold uppercase text-slate-500"><T>Positions</T></h3>
          {positions.length === 0 ? <p className="text-sm text-slate-400"><T>No positions.</T></p> : (
            <Table head={<><th className="th"><T>Title</T></th><th className="th"><T>Holder</T></th></>}>
              {positions.map((p) => (
                <tr key={p.id} className="hover:bg-slate-50">
                  <td className="td"><Link href={`/o/${p.id}`} className="text-indigo-600 hover:underline">{pickLabel(p.title) || p.code}</Link></td>
                  <td className="td">{p.holder ? <Mono>{p.holder.personId.slice(-6)}</Mono> : <span className="text-amber-600"><T>vacant</T></span>}</td>
                </tr>
              ))}
            </Table>
          )}
        </div>
      </div>
    </Card>
  );
}
