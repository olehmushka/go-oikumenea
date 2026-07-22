"use client";

// Religion workspace (M22 / D-Religion). Browse the recursive multi-faith taxonomy (religion → branch →
// tradition → sub-tradition → denomination) with rank/religion/text filters, create a taxon, and inspect
// a taxon's effective theism classification (nearest-declared-wins) + edit the tags declared on it.
// Per-unit religion profiles/classifications are managed from the unit object view.

import { useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { PageHeader, Card, Table, Mono, Pill } from "@/components/ui";
import { UnitSelect } from "@/components/UnitSelect";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
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
  const tr = useTg();
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
    api.religion.listTaxonRanks().then((r) => setRanks((r.taxonRanks ?? []) as unknown as Rank[])).catch(() => {});
    api.religion.listClassifications().then((r) => setClassifications((r.classifications ?? []) as unknown as Classification[])).catch(() => {});
    api.religion.listTaxa("religion", undefined, undefined, undefined, 100).then((r) => setReligions((r.taxa ?? []) as unknown as Taxon[])).catch(() => {});
  }, []);

  function reload() {
    const params = new URLSearchParams({ pageSize: "200" });
    if (rankFilter) params.set("rank", rankFilter);
    if (religionFilter) params.set("religion", religionFilter);
    if (query.trim()) params.set("query", query.trim());
    api.request<{ taxa: Taxon[] }>("GET", `/religion/v1/taxa?${params.toString()}`)
      .then((r) => setTaxa(r.taxa ?? []))
      .catch(setErr);
  }
  useEffect(reload, [rankFilter, religionFilter, query]);

  return (
    <div>
      <PageHeader
        title={<T>Religion</T>}
        description={<T>The multi-faith taxonomy (religion → branch → tradition → sub-tradition → denomination) and the religion-type classifications. Organizations reuse tenant units; per-unit faith profiles live on the unit object view.</T>}
      />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2 space-y-6">
          <Card>
            <div className="mb-3 flex flex-wrap items-end gap-2">
              <div>
                <label className="block text-xs text-slate-500"><T>Rank</T></label>
                <select className="input w-40" value={rankFilter} onChange={(e) => setRankFilter(e.target.value)}>
                  <option value="">{tr("all ranks")}</option>
                  {ranks.map((r) => <option key={r.id} value={r.code}>{pickLabel(r.name)}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs text-slate-500"><T>Religion</T></label>
                <select className="input w-44" value={religionFilter} onChange={(e) => setReligionFilter(e.target.value)}>
                  <option value="">{tr("all faiths")}</option>
                  {religions.map((r) => <option key={r.id} value={r.code}>{pickLabel(r.name)}</option>)}
                </select>
              </div>
              <div className="flex-1 min-w-[12rem]">
                <label className="block text-xs text-slate-500"><T>Search</T></label>
                <input className="input w-full" placeholder={tr("code or name")} value={query} onChange={(e) => setQuery(e.target.value)} />
              </div>
            </div>
            <TaxaTable taxa={taxa} selected={selected} onSelect={setSelected} />
          </Card>
          <CreateTaxon ranks={ranks} taxa={taxa} religions={religions} onCreated={reload} />
        </div>
        <div className="space-y-6">
          {selected ? <TaxonDetail taxon={selected} classifications={classifications} onChanged={reload} /> : (
            <Card><p className="text-sm text-slate-400"><T>Select a taxon to see its resolved classification and edit its theism tags.</T></p></Card>
          )}
        </div>
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-3">
        <ClergyGradesPanel />
        <AffiliationTypesPanel />
        <ClergyRosterPanel />
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-3">
        <SiteTypesPanel />
        <ServiceTypesPanel />
        <DiscoverySearchPanel />
      </div>

      <div className="mt-6">
        <UnitSitesPanel />
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
    api.religion.listClergyGrades()
      .then((r) => setGrades((r.clergyGrades ?? []) as unknown as ClergyGrade[]))
      .catch(setErr);
  }, []);
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Clergy grades</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      {grades.length === 0 ? <p className="text-sm text-slate-400"><T>No grades.</T></p> : (
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
    api.religion.listAffiliationTypes()
      .then((r) => setTypes((r.affiliationTypes ?? []) as unknown as AffiliationType[]))
      .catch(setErr);
  }, []);
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Affiliation types</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      {types.length === 0 ? <p className="text-sm text-slate-400"><T>No types.</T></p> : (
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
    api.request<{ credentials: ClergyCredential[] }>("GET", `/religion/v1/units/${unitId.trim()}/clergy-credentials`)
      .then((r) => setRows(r.credentials ?? []))
      .catch(setErr);
  }
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Clergy roster (by org unit)</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={lookup} className="mb-3 flex items-start gap-2">
        <div className="flex-1"><UnitSelect onChange={setUnitId} /></div>
        <button className="btn" type="submit" disabled={!unitId.trim()}><T>Lookup</T></button>
      </form>
      {rows === null ? <p className="text-sm text-slate-400"><T>Select an org unit.</T></p> : rows.length === 0 ? (
        <p className="text-sm text-slate-400"><T>No credentials conferred by this unit.</T></p>
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

// TaxaTable renders the taxonomy as an indented TREE (religion → branch → tradition → …). The flat list
// is assembled into a forest via parentId (a node whose parent isn't in the current — possibly filtered —
// set becomes a root), then walked in pre-order so children follow their parent. Rows with children carry
// a collapse toggle; the tree is fully expanded by default so the hierarchy is visible at a glance.
function TaxaTable({ taxa, selected, onSelect }: { taxa: Taxon[]; selected: Taxon | null; onSelect: (t: Taxon) => void }) {
  const tr = useTg();
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  const { roots, childrenOf } = useMemo(() => {
    const ids = new Set(taxa.map((t) => t.id));
    const childrenOf = new Map<string, Taxon[]>();
    const roots: Taxon[] = [];
    for (const t of taxa) {
      if (t.parentId && ids.has(t.parentId)) {
        const arr = childrenOf.get(t.parentId) ?? [];
        arr.push(t);
        childrenOf.set(t.parentId, arr);
      } else {
        roots.push(t);
      }
    }
    return { roots, childrenOf };
  }, [taxa]);

  const rows = useMemo(() => {
    const out: { taxon: Taxon; depth: number; hasChildren: boolean }[] = [];
    const walk = (nodes: Taxon[], depth: number) => {
      for (const t of nodes) {
        const kids = childrenOf.get(t.id) ?? [];
        out.push({ taxon: t, depth, hasChildren: kids.length > 0 });
        if (kids.length > 0 && !collapsed.has(t.id)) walk(kids, depth + 1);
      }
    };
    walk(roots, 0);
    return out;
  }, [roots, childrenOf, collapsed]);

  function toggle(id: string) {
    setCollapsed((s) => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });
  }

  if (taxa.length === 0) return <p className="text-sm text-slate-400"><T>No taxa match.</T></p>;
  return (
    <Table head={["Name", "Rank", "Code", "Wikidata"].map(tr)}>
      {rows.map(({ taxon: t, depth, hasChildren }) => (
        <tr key={t.id} className={`cursor-pointer hover:bg-slate-50 ${selected?.id === t.id ? "bg-slate-50" : ""}`} onClick={() => onSelect(t)}>
          <td className="py-1.5">
            <span className="inline-flex items-center" style={{ paddingLeft: `${depth * 1.25}rem` }}>
              {hasChildren ? (
                <button
                  type="button"
                  className="mr-1 w-4 shrink-0 text-slate-400 hover:text-slate-700"
                  aria-label={collapsed.has(t.id) ? tr("expand") : tr("collapse")}
                  onClick={(e) => { e.stopPropagation(); toggle(t.id); }}
                >
                  {collapsed.has(t.id) ? "▸" : "▾"}
                </button>
              ) : (
                <span className="mr-1 inline-block w-4 shrink-0" />
              )}
              {pickLabel(t.name)}
            </span>
          </td>
          <td><Pill>{t.rankCode}</Pill></td>
          <td><Mono>{t.code}</Mono></td>
          <td>{t.wikidataId ? <Mono>{t.wikidataId}</Mono> : <span className="text-slate-300">—</span>}</td>
        </tr>
      ))}
    </Table>
  );
}

function CreateTaxon({ ranks, taxa, religions, onCreated }: { ranks: Rank[]; taxa: Taxon[]; religions: Taxon[]; onCreated: () => void }) {
  const tr = useTg();
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
      await api.religion.createTaxon({
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
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Add taxon</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <input required className="input" placeholder={tr("name")} value={name} onChange={(e) => setName(e.target.value)} />
        <input required className="input" placeholder={tr("auto from name")} value={codeValue} onChange={(e) => { setCode(e.target.value); setCodeTouched(true); }} />
        <select required className="input" value={rankId} onChange={(e) => setRankId(e.target.value)}>
          <option value="">{tr("— rank —")}</option>
          {ranks.map((r) => <option key={r.id} value={r.id}>{pickLabel(r.name)}</option>)}
        </select>
        <select className="input" value={parentId} onChange={(e) => setParentId(e.target.value)}>
          <option value="">{tr("— root religion (no parent) —")}</option>
          {parents.map((p) => <option key={p.id} value={p.id}>{pickLabel(p.name)} ({p.rankCode})</option>)}
        </select>
        <div className="sm:col-span-2">
          <button type="submit" className="btn" disabled={busy}>{busy ? <T>Adding…</T> : <T>Add taxon</T>}</button>
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
    api.religion.getEffectiveClassifications(taxon.id)
      .then((r) => setEffective((r.classifications ?? []) as unknown as Classification[]))
      .catch(setErr);
    setSelectedIds([]);
  }, [taxon.id]);

  function toggle(id: string) {
    setSelectedIds((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));
  }

  async function save() {
    setBusy(true); setErr(null);
    try {
      const r = await api.religion.setTaxonClassifications(taxon.id, { classificationIds: selectedIds });
      setEffective((r.classifications ?? []) as unknown as Classification[]);
      onChanged();
    } catch (e) { setErr(e); } finally { setBusy(false); }
  }

  return (
    <Card>
      <h2 className="mb-1 text-sm font-semibold text-slate-700">{pickLabel(taxon.name)}</h2>
      <p className="mb-3 text-xs text-slate-400"><Pill>{taxon.rankCode}</Pill> <Mono>{taxon.code}</Mono></p>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <div className="mb-4">
        <div className="mb-1 text-xs font-medium text-slate-500"><T>Effective religion-type (resolved)</T></div>
        {effective.length === 0 ? <p className="text-sm text-slate-400"><T>none (no ancestor declares one)</T></p> : (
          <div className="flex flex-wrap gap-1">{effective.map((c) => <Pill key={c.id}>{pickLabel(c.name)}</Pill>)}</div>
        )}
      </div>
      <div className="mb-2 text-xs font-medium text-slate-500"><T>Declare tags on this taxon (overrides inherited)</T></div>
      <div className="mb-3 flex flex-wrap gap-2">
        {classifications.map((c) => (
          <label key={c.id} className="flex items-center gap-1 text-sm">
            <input type="checkbox" checked={selectedIds.includes(c.id)} onChange={() => toggle(c.id)} />
            {pickLabel(c.name)}
          </label>
        ))}
      </div>
      <button className="btn" onClick={save} disabled={busy}>{busy ? <T>Saving…</T> : <T>Set declared tags</T>}</button>
    </Card>
  );
}

// ── M25 discovery: site/service catalogs, search, and per-unit sites/aliases ─────────────────────────

type SiteType = { id: string; code: string; name: LocaleMap; traditionTaxonId?: string };
type ServiceType = { id: string; code: string; name: LocaleMap; traditionTaxonId?: string };
type Site = {
  id: string;
  orgUnitId: string;
  locationId: string;
  siteTypeId: string;
  siteTypeCode: string;
  siteTypeName: LocaleMap;
  visibility: string;
  publicPrecision: string;
  isPrimary: boolean;
  latitude: number;
  longitude: number;
};
type DiscoverySite = {
  id: string;
  orgUnitId: string;
  siteTypeCode: string;
  siteTypeName: LocaleMap;
  publicPrecision: string;
  isPrimary: boolean;
  latitude?: number;
  longitude?: number;
};
type Alias = { id: string; aliasText: string; aliasType: string; locale?: string };

function CatalogPanel<R extends { id: string; code: string; name: LocaleMap }>({ title, path, dataKey }: { title: string; path: string; dataKey: string }) {
  const tr = useTg();
  const [rows, setRows] = useState<R[]>([]);
  const [err, setErr] = useState<unknown>(null);
  useEffect(() => {
    api.request<Record<string, R[]>>("GET", path).then((r) => setRows(r[dataKey] ?? [])).catch(setErr);
  }, [path, dataKey]);
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700">{tr(title)}</h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      {rows.length === 0 ? <p className="text-sm text-slate-400"><T>None.</T></p> : (
        <ul className="space-y-0.5 text-sm text-slate-700">
          {rows.map((r) => (
            <li key={r.id} className="flex items-center justify-between gap-2">
              <span>{pickLabel(r.name)}</span>
              <Mono>{r.code}</Mono>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

function SiteTypesPanel() {
  return <CatalogPanel<SiteType> title="Site types" path="/religion/v1/site-types" dataKey="siteTypes" />;
}
function ServiceTypesPanel() {
  return <CatalogPanel<ServiceType> title="Service types" path="/religion/v1/service-types" dataKey="serviceTypes" />;
}
// (title strings are translated inside CatalogPanel via the glossary)

const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

// DiscoverySearchPanel runs the public discovery search (radius + religion/language/day/online filters)
// and shows hits with COARSENED coordinates per each site's publish precision.
function DiscoverySearchPanel() {
  const tr = useTg();
  const [lat, setLat] = useState("");
  const [lng, setLng] = useState("");
  const [radiusM, setRadiusM] = useState("5000");
  const [language, setLanguage] = useState("");
  const [day, setDay] = useState("");
  const [online, setOnline] = useState(false);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<DiscoverySite[] | null>(null);
  const [err, setErr] = useState<unknown>(null);

  function search(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    const p = new URLSearchParams();
    if (lat && lng) { p.set("lat", lat); p.set("lng", lng); if (radiusM) p.set("radiusM", radiusM); }
    if (language.trim()) p.set("language", language.trim());
    if (day !== "") p.set("dayOfWeek", day);
    if (online) p.set("onlineOnly", "true");
    if (query.trim()) p.set("query", query.trim());
    api.request<{ sites: DiscoverySite[] }>("GET", `/religion/v1/discovery/sites?${p.toString()}`)
      .then((r) => setHits(r.sites ?? []))
      .catch(setErr);
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Discovery search</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={search} className="grid grid-cols-2 gap-2">
        <input className="input" placeholder={tr("lat")} value={lat} onChange={(e) => setLat(e.target.value)} />
        <input className="input" placeholder={tr("lng")} value={lng} onChange={(e) => setLng(e.target.value)} />
        <input className="input" placeholder={tr("radius m")} value={radiusM} onChange={(e) => setRadiusM(e.target.value)} />
        <input className="input" placeholder={tr("language (ISO 639-3)")} value={language} onChange={(e) => setLanguage(e.target.value)} />
        <select className="input" value={day} onChange={(e) => setDay(e.target.value)}>
          <option value="">{tr("any day")}</option>
          {DAYS.map((d, i) => <option key={d} value={i}>{tr(d)}</option>)}
        </select>
        <input className="input" placeholder={tr("name / alias")} value={query} onChange={(e) => setQuery(e.target.value)} />
        <label className="col-span-2 flex items-center gap-1 text-sm text-slate-600">
          <input type="checkbox" checked={online} onChange={(e) => setOnline(e.target.checked)} /> {tr("online / hybrid only")}
        </label>
        <div className="col-span-2"><button className="btn" type="submit"><T>Search</T></button></div>
      </form>
      {hits !== null && (
        <div className="mt-3">
          {hits.length === 0 ? <p className="text-sm text-slate-400"><T>No public sites match.</T></p> : (
            <ul className="space-y-0.5 text-sm text-slate-700">
              {hits.map((h) => (
                <li key={h.id} className="flex items-center justify-between gap-2">
                  <span>{pickLabel(h.siteTypeName) || h.siteTypeCode}{h.isPrimary ? " ★" : ""}</span>
                  <Mono>{h.latitude != null ? `${h.latitude.toFixed(4)}, ${h.longitude?.toFixed(4)} (${h.publicPrecision})` : `hidden`}</Mono>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </Card>
  );
}

// UnitSitesPanel manages an org unit's sites (list + add over a shared location) and its search-only
// aliases. Per-site schedule management is reachable from the unit object view; this is the directory editor.
function UnitSitesPanel() {
  const tr = useTg();
  const [unitId, setUnitId] = useState("");
  const [sites, setSites] = useState<Site[] | null>(null);
  const [aliases, setAliases] = useState<Alias[]>([]);
  const [siteTypes, setSiteTypes] = useState<SiteType[]>([]);
  const [err, setErr] = useState<unknown>(null);

  // add-site form
  const [locationId, setLocationId] = useState("");
  const [siteTypeId, setSiteTypeId] = useState("");
  const [precision, setPrecision] = useState("exact");
  const [isPrimary, setIsPrimary] = useState(false);
  // add-alias form
  const [aliasText, setAliasText] = useState("");
  const [aliasType, setAliasType] = useState("transliteration");

  useEffect(() => {
    api.religion.listSiteTypes().then((r) => setSiteTypes((r.siteTypes ?? []) as unknown as SiteType[])).catch(() => {});
  }, []);

  function load() {
    if (!unitId.trim()) return;
    setErr(null);
    const u = unitId.trim();
    api.religion.listUnitSites(u).then((r) => setSites((r.sites ?? []) as unknown as Site[])).catch(setErr);
    api.religion.listUnitAliases(u).then((r) => setAliases((r.aliases ?? []) as unknown as Alias[])).catch(() => {});
  }

  async function addSite(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      await api.religion.createSite(unitId.trim(), {
        locationId: locationId.trim(), siteTypeId, publicPrecision: precision, isPrimary,
      });
      setLocationId(""); setIsPrimary(false);
      load();
    } catch (e) { setErr(e); }
  }

  async function addAlias(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      await api.religion.createAlias(unitId.trim(), { aliasText: aliasText.trim(), aliasType } as never);
      setAliasText("");
      load();
    } catch (e) { setErr(e); }
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Sites &amp; aliases (by org unit)</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <div className="mb-3 flex items-start gap-2">
        <div className="flex-1"><UnitSelect onChange={setUnitId} /></div>
        <button className="btn" onClick={load} disabled={!unitId.trim()}><T>Load</T></button>
      </div>
      {sites !== null && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <div>
            <div className="mb-1 text-xs font-medium text-slate-500"><T>Sites</T></div>
            {sites.length === 0 ? <p className="text-sm text-slate-400"><T>No sites.</T></p> : (
              <Table head={["Type", "Visibility", "Precision", "Primary", "Coord"].map(tr)}>
                {sites.map((s) => (
                  <tr key={s.id}>
                    <td className="py-1.5">{pickLabel(s.siteTypeName) || s.siteTypeCode}</td>
                    <td><Pill>{s.visibility}</Pill></td>
                    <td><Mono>{s.publicPrecision}</Mono></td>
                    <td>{s.isPrimary ? "★" : ""}</td>
                    <td><Mono>{s.latitude.toFixed(4)}, {s.longitude.toFixed(4)}</Mono></td>
                  </tr>
                ))}
              </Table>
            )}
            <form onSubmit={addSite} className="mt-3 grid grid-cols-1 gap-2">
              <input required className="input" placeholder={tr("location RID")} value={locationId} onChange={(e) => setLocationId(e.target.value)} />
              <select required className="input" value={siteTypeId} onChange={(e) => setSiteTypeId(e.target.value)}>
                <option value="">{tr("— site type —")}</option>
                {siteTypes.map((t) => <option key={t.id} value={t.id}>{pickLabel(t.name)} ({t.code})</option>)}
              </select>
              <select className="input" value={precision} onChange={(e) => setPrecision(e.target.value)}>
                {["exact", "street", "neighborhood", "city", "hidden"].map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
              <label className="flex items-center gap-1 text-sm text-slate-600">
                <input type="checkbox" checked={isPrimary} onChange={(e) => setIsPrimary(e.target.checked)} /> {tr("primary site")}
              </label>
              <button className="btn" type="submit" disabled={!unitId.trim()}><T>Add site</T></button>
            </form>
          </div>
          <div>
            <div className="mb-1 text-xs font-medium text-slate-500"><T>Aliases (search-only)</T></div>
            {aliases.length === 0 ? <p className="text-sm text-slate-400"><T>No aliases.</T></p> : (
              <ul className="space-y-0.5 text-sm text-slate-700">
                {aliases.map((a) => (
                  <li key={a.id} className="flex items-center justify-between gap-2">
                    <span>{a.aliasText}</span>
                    <Pill>{a.aliasType}</Pill>
                  </li>
                ))}
              </ul>
            )}
            <form onSubmit={addAlias} className="mt-3 grid grid-cols-1 gap-2">
              <input required className="input" placeholder={tr("alias text")} value={aliasText} onChange={(e) => setAliasText(e.target.value)} />
              <select className="input" value={aliasType} onChange={(e) => setAliasType(e.target.value)}>
                {["nickname", "abbreviation", "historical", "misspelling", "transliteration"].map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
              <button className="btn" type="submit" disabled={!unitId.trim()}><T>Add alias</T></button>
            </form>
          </div>
        </div>
      )}
    </Card>
  );
}
