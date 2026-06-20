"use client";

// Religion workspace (M22 / D-Religion). Browse the recursive multi-faith taxonomy (religion → branch →
// tradition → sub-tradition → denomination) with rank/religion/text filters, create a taxon, and inspect
// a taxon's effective theism classification (nearest-declared-wins) + edit the tags declared on it.
// Per-unit religion profiles/classifications are managed from the unit object view.

import { useEffect, useMemo, useRef, useState } from "react";
import { mutate } from "@/lib/api/client";
import { bffGet } from "@/lib/api/browser";
import { PageHeader, Card, Table, Mono, Pill } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { newSuffix, slugify } from "@/lib/code";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Rank = { id: string; code: string; name: LocaleMap; ordinal: number };
type Classification = { id: string; code: string; name: LocaleMap };
type Taxon = {
  id: string;
  code: string;
  name: LocaleMap;
  rankId: string;
  rankCode: string;
  parentId?: string;
  religionId?: string;
  wikidataId?: string;
  depth?: number;
};

export default function ReligionPage() {
  const [ranks, setRanks] = useState<Rank[]>([]);
  const [classifications, setClassifications] = useState<Classification[]>([]);
  const [religions, setReligions] = useState<Taxon[]>([]);
  const [taxa, setTaxa] = useState<Taxon[]>([]);
  const [rankFilter, setRankFilter] = useState("");
  const [religionFilter, setReligionFilter] = useState("");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Taxon | null>(null);
  const [err, setErr] = useState<unknown>(null);

  useEffect(() => {
    bffGet<{ taxonRanks: Rank[] }>("/religion/v1/taxon-ranks").then((r) => setRanks(r.taxonRanks ?? [])).catch(() => {});
    bffGet<{ classifications: Classification[] }>("/religion/v1/classifications").then((r) => setClassifications(r.classifications ?? [])).catch(() => {});
    bffGet<{ taxa: Taxon[] }>("/religion/v1/taxa?rank=religion&pageSize=100").then((r) => setReligions(r.taxa ?? [])).catch(() => {});
  }, []);

  function reload() {
    const params = new URLSearchParams({ pageSize: "200" });
    if (rankFilter) params.set("rank", rankFilter);
    if (religionFilter) params.set("religion", religionFilter);
    if (query.trim()) params.set("query", query.trim());
    bffGet<{ taxa: Taxon[] }>(`/religion/v1/taxa?${params.toString()}`)
      .then((r) => setTaxa(r.taxa ?? []))
      .catch(setErr);
  }
  useEffect(reload, [rankFilter, religionFilter, query]);

  return (
    <div>
      <PageHeader
        title="Religion"
        description="The multi-faith taxonomy (religion → branch → tradition → sub-tradition → denomination) and the religion-type classifications. Organizations reuse tenant units; per-unit faith profiles live on the unit object view."
      />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2 space-y-6">
          <Card>
            <div className="mb-3 flex flex-wrap items-end gap-2">
              <div>
                <label className="block text-xs text-slate-500">Rank</label>
                <select className="input w-40" value={rankFilter} onChange={(e) => setRankFilter(e.target.value)}>
                  <option value="">all ranks</option>
                  {ranks.map((r) => <option key={r.id} value={r.code}>{pickLabel(r.name)}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs text-slate-500">Religion</label>
                <select className="input w-44" value={religionFilter} onChange={(e) => setReligionFilter(e.target.value)}>
                  <option value="">all faiths</option>
                  {religions.map((r) => <option key={r.id} value={r.code}>{pickLabel(r.name)}</option>)}
                </select>
              </div>
              <div className="flex-1 min-w-[12rem]">
                <label className="block text-xs text-slate-500">Search</label>
                <input className="input w-full" placeholder="code or name" value={query} onChange={(e) => setQuery(e.target.value)} />
              </div>
            </div>
            <TaxaTable taxa={taxa} selected={selected} onSelect={setSelected} />
          </Card>
          <CreateTaxon ranks={ranks} taxa={taxa} religions={religions} onCreated={reload} />
        </div>
        <div className="space-y-6">
          {selected ? <TaxonDetail taxon={selected} classifications={classifications} onChanged={reload} /> : (
            <Card><p className="text-sm text-slate-400">Select a taxon to see its resolved classification and edit its theism tags.</p></Card>
          )}
        </div>
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-3">
        <ClergyGradesPanel />
        <AffiliationTypesPanel />
        <ClergyRosterPanel />
      </div>
    </div>
  );
}

// ── M23/M24 reference catalogs + clergy roster ──────────────────────────────

type ClergyGrade = { id: string; code: string; name: LocaleMap; ordinal: number; traditionTaxonId?: string };
type AffiliationType = { id: string; code: string; name: LocaleMap; traditionTaxonId?: string };
type ClergyCredential = {
  id: string;
  personId: string;
  gradeCode: string;
  gradeName: LocaleMap;
  orgUnitId: string;
  status: string;
  grantedOn?: string;
};

function ClergyGradesPanel() {
  const [grades, setGrades] = useState<ClergyGrade[]>([]);
  const [err, setErr] = useState<unknown>(null);
  useEffect(() => {
    bffGet<{ clergyGrades: ClergyGrade[] }>("/religion/v1/clergy-grades")
      .then((r) => setGrades(r.clergyGrades ?? []))
      .catch(setErr);
  }, []);
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">Clergy grades</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      {grades.length === 0 ? <p className="text-sm text-slate-400">No grades.</p> : (
        <ul className="space-y-0.5 text-sm text-slate-700">
          {grades.map((g) => (
            <li key={g.id} className="flex items-center justify-between gap-2">
              <span>{pickLabel(g.name)}</span>
              <Mono>{g.code}</Mono>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function AffiliationTypesPanel() {
  const [types, setTypes] = useState<AffiliationType[]>([]);
  const [err, setErr] = useState<unknown>(null);
  useEffect(() => {
    bffGet<{ affiliationTypes: AffiliationType[] }>("/religion/v1/affiliation-types")
      .then((r) => setTypes(r.affiliationTypes ?? []))
      .catch(setErr);
  }, []);
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">Affiliation types</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      {types.length === 0 ? <p className="text-sm text-slate-400">No types.</p> : (
        <ul className="space-y-0.5 text-sm text-slate-700">
          {types.map((t) => (
            <li key={t.id} className="flex items-center justify-between gap-2">
              <span>{pickLabel(t.name)}</span>
              <Mono>{t.code}</Mono>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

// ClergyRosterPanel lists the people holding a credential conferred by a given organization unit.
function ClergyRosterPanel() {
  const [unitId, setUnitId] = useState("");
  const [rows, setRows] = useState<ClergyCredential[] | null>(null);
  const [err, setErr] = useState<unknown>(null);
  function lookup(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!unitId.trim()) return;
    bffGet<{ credentials: ClergyCredential[] }>(`/religion/v1/units/${unitId.trim()}/clergy-credentials`)
      .then((r) => setRows(r.credentials ?? []))
      .catch(setErr);
  }
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">Clergy roster (by org unit)</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={lookup} className="mb-3 flex gap-2">
        <input className="input flex-1" placeholder="org unit RID" value={unitId} onChange={(e) => setUnitId(e.target.value)} />
        <button className="btn" type="submit">Lookup</button>
      </form>
      {rows === null ? <p className="text-sm text-slate-400">Enter a unit RID.</p> : rows.length === 0 ? (
        <p className="text-sm text-slate-400">No credentials conferred by this unit.</p>
      ) : (
        <ul className="space-y-0.5 text-sm text-slate-700">
          {rows.map((c) => (
            <li key={c.id} className="flex items-center justify-between gap-2">
              <span>
                {pickLabel(c.gradeName) || c.gradeCode} · <Mono>{c.personId.slice(-8)}</Mono>
              </span>
              <Pill tone={c.status === "active" ? "green" : "slate"}>{c.status}</Pill>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function TaxaTable({ taxa, selected, onSelect }: { taxa: Taxon[]; selected: Taxon | null; onSelect: (t: Taxon) => void }) {
  if (taxa.length === 0) return <p className="text-sm text-slate-400">No taxa match.</p>;
  return (
    <Table head={["Name", "Rank", "Code", "Wikidata"]}>
      {taxa.map((t) => (
        <tr key={t.id} className={`cursor-pointer hover:bg-slate-50 ${selected?.id === t.id ? "bg-slate-50" : ""}`} onClick={() => onSelect(t)}>
          <td className="py-1.5">{pickLabel(t.name)}</td>
          <td><Pill>{t.rankCode}</Pill></td>
          <td><Mono>{t.code}</Mono></td>
          <td>{t.wikidataId ? <Mono>{t.wikidataId}</Mono> : <span className="text-slate-300">—</span>}</td>
        </tr>
      ))}
    </Table>
  );
}

function CreateTaxon({ ranks, taxa, religions, onCreated }: { ranks: Rank[]; taxa: Taxon[]; religions: Taxon[]; onCreated: () => void }) {
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  const [rankId, setRankId] = useState("");
  const [parentId, setParentId] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const suffix = useRef(newSuffix());
  const slug = slugify(name);
  const codeValue = codeTouched ? code : slug ? `${slug}-${suffix.current}` : "";
  const parents = useMemo(() => [...religions, ...taxa].filter((v, i, a) => a.findIndex((x) => x.id === v.id) === i), [religions, taxa]);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true); setErr(null);
    try {
      await mutate("POST", "/religion/v1/taxa", {
        code: codeValue.trim(),
        name: name.trim(),
        rankId,
        parentId: parentId || undefined,
      });
      setName(""); setCode(""); setCodeTouched(false); setParentId(""); suffix.current = newSuffix();
      onCreated();
    } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">Add taxon</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <input required className="input" placeholder="name" value={name} onChange={(e) => setName(e.target.value)} />
        <input required className="input" placeholder="auto from name" value={codeValue} onChange={(e) => { setCode(e.target.value); setCodeTouched(true); }} />
        <select required className="input" value={rankId} onChange={(e) => setRankId(e.target.value)}>
          <option value="">— rank —</option>
          {ranks.map((r) => <option key={r.id} value={r.id}>{pickLabel(r.name)}</option>)}
        </select>
        <select className="input" value={parentId} onChange={(e) => setParentId(e.target.value)}>
          <option value="">— root religion (no parent) —</option>
          {parents.map((p) => <option key={p.id} value={p.id}>{pickLabel(p.name)} ({p.rankCode})</option>)}
        </select>
        <div className="sm:col-span-2">
          <button type="submit" className="btn" disabled={busy}>{busy ? "Adding…" : "Add taxon"}</button>
        </div>
      </form>
    </Card>
  );
}

function TaxonDetail({ taxon, classifications, onChanged }: { taxon: Taxon; classifications: Classification[]; onChanged: () => void }) {
  const [effective, setEffective] = useState<Classification[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);

  useEffect(() => {
    bffGet<{ classifications: Classification[] }>(`/religion/v1/taxa/${taxon.id}/effective-classifications`)
      .then((r) => setEffective(r.classifications ?? []))
      .catch(setErr);
    setSelectedIds([]);
  }, [taxon.id]);

  function toggle(id: string) {
    setSelectedIds((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));
  }

  async function save() {
    setBusy(true); setErr(null);
    try {
      const r = await mutate<{ classifications: Classification[] }>("PUT", `/religion/v1/taxa/${taxon.id}/classifications`, { classificationIds: selectedIds });
      setEffective(r.classifications ?? []);
      onChanged();
    } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  return (
    <Card>
      <h2 className="mb-1 text-sm font-semibold text-slate-700">{pickLabel(taxon.name)}</h2>
      <p className="mb-3 text-xs text-slate-400"><Pill>{taxon.rankCode}</Pill> <Mono>{taxon.code}</Mono></p>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <div className="mb-4">
        <div className="mb-1 text-xs font-medium text-slate-500">Effective religion-type (resolved)</div>
        {effective.length === 0 ? <p className="text-sm text-slate-400">none (no ancestor declares one)</p> : (
          <div className="flex flex-wrap gap-1">{effective.map((c) => <Pill key={c.id}>{pickLabel(c.name)}</Pill>)}</div>
        )}
      </div>
      <div className="mb-2 text-xs font-medium text-slate-500">Declare tags on this taxon (overrides inherited)</div>
      <div className="mb-3 flex flex-wrap gap-2">
        {classifications.map((c) => (
          <label key={c.id} className="flex items-center gap-1 text-sm">
            <input type="checkbox" checked={selectedIds.includes(c.id)} onChange={() => toggle(c.id)} />
            {pickLabel(c.name)}
          </label>
        ))}
      </div>
      <button className="btn" onClick={save} disabled={busy}>{busy ? "Saving…" : "Set declared tags"}</button>
    </Card>
  );
}
