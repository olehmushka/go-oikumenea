"use client";

// External-organizations workspace (M30 / D-ExternalOrgs). External organizations a person is tied to
// but the deploying org neither owns nor commands — parties, government bodies, foreign military, NGOs,
// lobbying registrants. Catalog-typed, provisional/resolved + attribution, hermenea-fed (Wikidata). The
// kind catalog is managed here too. These are external reference data, independent of the deploying
// org's units, and never enter the PDP graph.
//
// M58 ticket 2 moved BROWSING out. /explore/external_organization is the registry's real reader: six
// facet filters, keyset paging that does not drop its token, and a dashboard over the same filter set.
// What stays here is EDITING — creation, the kind catalog, and the per-row merge/edit/delete forms,
// which are richer than the generic action runner (a merge has to pick its surviving org).
//
// The table below is therefore a bounded EDIT surface, not a listing: it shows one page and says so.
// That is the difference from the pre-M58 page, which fetched 200 rows, offered two of the six
// filters client-side, and silently dropped the next-page token — presenting a truncation as a
// registry.

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { PageHeader, Card, Table, Mono } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Kind = { id: string; code: string; name: LocaleMap; status: string; sortOrder?: number };
type Org = {
  id: string; kindId: string; kindLabel?: string; name: LocaleMap; code?: string;
  countryId?: string; countryLabel?: string; wikidataId?: string;
  status: string; source: string; confidence: string;
};

const label = (m: LocaleMap, fallback: string) => pickLabel(m) || fallback;

// The edit surface's page size. Small on purpose: this is a working set to act on, not a listing —
// the listing is /explore/external_organization.
const EDIT_PAGE = 50;

export default function ExternalOrgsPage() {
  const tr = useTg();
  const [kinds, setKinds] = useState<Kind[]>([]);
  const [orgs, setOrgs] = useState<Org[]>([]);
  const [truncated, setTruncated] = useState(false);
  const [err, setErr] = useState<unknown>(null);

  // One page, and the next-page token is READ rather than discarded — its presence is what the notice
  // below reports. Filtering is deliberately not offered here; it is the explorer's job now.
  function reload() {
    api.externalOrg
      .listExternalOrgs(undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined, EDIT_PAGE)
      .then((r) => {
        setOrgs((r.orgs ?? []) as unknown as Org[]);
        setTruncated(Boolean(r.nextPageToken));
      })
      .catch(setErr);
  }
  function reloadKinds() {
    api.externalOrg.listExternalOrgKinds().then((r) => setKinds((r.kinds ?? []) as unknown as Kind[])).catch(() => {});
  }
  useEffect(() => { reloadKinds(); }, []);
  useEffect(() => { reload(); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, []);

  return (
    <div>
      <PageHeader
        title={<T>External organizations</T>}
        description={<T>A registry of external organizations a person is tied to — parties, government bodies, foreign military, NGOs, lobbying registrants. External reference data, independent of the deploying org's units; never part of the PDP graph.</T>}
      />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}

      <div className="grid gap-6 lg:grid-cols-2">
        <CreateOrg kinds={kinds} onCreated={reload} setErr={setErr} />
        <KindsCard kinds={kinds} onChanged={reloadKinds} setErr={setErr} />
      </div>

      <Card className="mt-6">
        <div className="mb-2 flex items-center gap-3">
          <h2 className="text-sm font-semibold text-slate-900"><T>Edit</T></h2>
          <Link href="/explore/external_organization" className="ml-auto text-xs text-indigo-600 hover:underline">
            <T>Browse, filter and chart the whole registry →</T>
          </Link>
        </div>
        <p className="mb-3 text-xs text-slate-500">
          {truncated
            ? tr("The first page only — there are more. Use the explorer to find a specific organization; this table is here to edit the ones in front of you.")
            : tr("Every organization in the registry. Use the explorer to filter or chart them.")}
        </p>
        <Table head={<><th className="th"><T>Name</T></th><th className="th"><T>Kind</T></th><th className="th"><T>Country</T></th><th className="th">Wikidata</th><th className="th"><T>Status</T></th><th className="th"><T>Source</T></th><th className="th"></th></>}>
          {orgs.map((o) => (
            <tr key={o.id} className="border-t">
              <td className="py-1"><Link href={`/o/${o.id}`} className="text-indigo-600 hover:underline">{label(o.name, o.id.slice(-8))}</Link></td>
              <td>{o.kindLabel || "—"}</td>
              <td>{o.countryLabel || "—"}</td>
              <td>{o.wikidataId ? <Mono>{o.wikidataId}</Mono> : "—"}</td>
              <td>{o.status}</td>
              <td className="text-slate-500">{o.source}</td>
              <td>
                <span className="flex items-center gap-2">
                  <Merge org={o} orgs={orgs} onDone={reload} setErr={setErr} />
                  <EditOrg org={o} kinds={kinds} onDone={reload} setErr={setErr} />
                  <DeleteOrg org={o} onDone={reload} setErr={setErr} />
                </span>
              </td>
            </tr>
          ))}
          {orgs.length === 0 ? <tr><td colSpan={7} className="py-2 text-slate-400"><T>No organizations.</T></td></tr> : null}
        </Table>
      </Card>
    </div>
  );
}

function CreateOrg({ kinds, onCreated, setErr }: { kinds: Kind[]; onCreated: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [kindId, setKindId] = useState("");
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [wikidataId, setWikidataId] = useState("");
  const [countryId, setCountryId] = useState("");
  const [provisional, setProvisional] = useState(false);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!kindId || !name.trim()) return;
    api.externalOrg
      .createExternalOrg({
        kindId,
        name: name.trim(),
        code: code.trim() || undefined,
        wikidataId: wikidataId.trim() || undefined,
        countryId: countryId || undefined,
        status: provisional ? "provisional" : undefined,
      })
      .then(() => { setName(""); setCode(""); setWikidataId(""); onCreated(); })
      .catch(setErr);
  }

  return (
    <Card>
      <h2 className="mb-2 text-sm font-semibold text-slate-900"><T>New organization</T></h2>
      <form onSubmit={submit} className="space-y-2 text-sm">
        <select className="input" value={kindId} onChange={(e) => setKindId(e.target.value)} required>
          <option value="">— kind —</option>
          {kinds.map((k) => <option key={k.id} value={k.id}>{label(k.name, k.code)}</option>)}
        </select>
        <input className="input" placeholder={tr("name")} value={name} onChange={(e) => setName(e.target.value)} required />
        <input className="input" placeholder={tr("code (optional)")} value={code} onChange={(e) => setCode(e.target.value)} />
        <input className="input" placeholder={tr("Wikidata Q-id (optional)")} value={wikidataId} onChange={(e) => setWikidataId(e.target.value)} />
        <select className="input" value={countryId} onChange={(e) => setCountryId(e.target.value)}>
          <CountryOptions />
        </select>
        <label className="flex items-center gap-2 text-xs text-slate-600">
          <input type="checkbox" checked={provisional} onChange={(e) => setProvisional(e.target.checked)} />
          <T>provisional stub (awaiting merge)</T>
        </label>
        <button className="btn-primary" type="submit"><T>Create</T></button>
      </form>
    </Card>
  );
}

function KindsCard({ kinds, onChanged, setErr }: { kinds: Kind[]; onChanged: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [code, setCode] = useState("");
  const [name, setName] = useState("");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!code.trim() || !name.trim()) return;
    api.externalOrg.upsertExternalOrgKind({ code: code.trim(), name: name.trim() })
      .then(() => { setCode(""); setName(""); onChanged(); })
      .catch(setErr);
  }

  return (
    <Card>
      <h2 className="mb-2 text-sm font-semibold text-slate-900"><T>Kind catalog</T></h2>
      <ul className="mb-3 space-y-1 text-sm">
        {kinds.map((k) => (
          <li key={k.id} className="flex items-center gap-2">
            <Mono>{k.code}</Mono>
            <span className="text-slate-500">{label(k.name, k.code)}</span>
          </li>
        ))}
      </ul>
      <form onSubmit={submit} className="flex gap-2 text-sm">
        <input className="input" placeholder={tr("code")} value={code} onChange={(e) => setCode(e.target.value)} />
        <input className="input" placeholder={tr("name")} value={name} onChange={(e) => setName(e.target.value)} />
        <button className="btn-secondary" type="submit"><T>Upsert</T></button>
      </form>
    </Card>
  );
}

// Merge resolves a provisional stub into a resolved canonical org.
function Merge({ org, orgs, onDone, setErr }: { org: Org; orgs: Org[]; onDone: () => void; setErr: (e: unknown) => void }) {
  const [open, setOpen] = useState(false);
  const [intoId, setIntoId] = useState("");
  if (org.status !== "provisional") return null;
  const targets = orgs.filter((o) => o.id !== org.id && o.status === "resolved");

  function submit() {
    if (!intoId) return;
    api.externalOrg.mergeExternalOrg(org.id, { intoOrgId: intoId }).then(() => { setOpen(false); onDone(); }).catch(setErr);
  }

  if (!open) return <button className="text-xs text-indigo-600 hover:underline" onClick={() => setOpen(true)}><T>Resolve</T></button>;
  return (
    <span className="flex items-center gap-1">
      <select className="input w-auto text-xs" value={intoId} onChange={(e) => setIntoId(e.target.value)}>
        <option value="">— into —</option>
        {targets.map((o) => <option key={o.id} value={o.id}>{label(o.name, o.id.slice(-8))}</option>)}
      </select>
      <button className="text-xs text-indigo-600 hover:underline" onClick={submit}><T>Merge</T></button>
    </span>
  );
}

// EditOrg edits name/code/kind/wikidata inline (updateExternalOrg; omitted fields unchanged).
function EditOrg({ org, kinds, onDone, setErr }: { org: Org; kinds: Kind[]; onDone: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [kindId, setKindId] = useState(org.kindId);
  const [name, setName] = useState(label(org.name, ""));
  const [code, setCode] = useState(org.code ?? "");
  const [wikidataId, setWikidataId] = useState(org.wikidataId ?? "");

  function submit() {
    if (!name.trim()) return;
    api.externalOrg
      .updateExternalOrg(org.id, {
        kindId: kindId || undefined,
        name: name.trim(),
        code: code.trim() || undefined,
        wikidataId: wikidataId.trim() || undefined,
      })
      .then(() => { setOpen(false); onDone(); })
      .catch(setErr);
  }

  if (!open) return <button className="text-xs text-indigo-600 hover:underline" onClick={() => setOpen(true)}><T>Edit</T></button>;
  return (
    <span className="flex flex-wrap items-center gap-1">
      <select className="input w-auto text-xs" value={kindId} onChange={(e) => setKindId(e.target.value)}>
        {kinds.map((k) => <option key={k.id} value={k.id}>{label(k.name, k.code)}</option>)}
      </select>
      <input className="input w-28 text-xs" value={name} onChange={(e) => setName(e.target.value)} placeholder={tr("name")} />
      <input className="input w-20 text-xs" value={code} onChange={(e) => setCode(e.target.value)} placeholder={tr("code")} />
      <input className="input w-24 text-xs" value={wikidataId} onChange={(e) => setWikidataId(e.target.value)} placeholder={tr("Q-id")} />
      <button className="text-xs text-indigo-600 hover:underline" onClick={submit}><T>Save</T></button>
      <button className="text-xs text-slate-400 hover:underline" onClick={() => setOpen(false)}><T>Cancel</T></button>
    </span>
  );
}

// DeleteOrg soft-deletes an external org (deleteExternalOrg).
function DeleteOrg({ org, onDone, setErr }: { org: Org; onDone: () => void; setErr: (e: unknown) => void }) {
  const [busy, setBusy] = useState(false);
  function del() {
    if (!window.confirm("Delete this external organization?")) return;
    setBusy(true);
    api.externalOrg.deleteExternalOrg(org.id).then(() => onDone()).catch(setErr).finally(() => setBusy(false));
  }
  return (
    <button className="text-xs text-red-600 hover:underline disabled:opacity-50" disabled={busy} onClick={del}>
      {busy ? <T>Deleting…</T> : <T>Delete</T>}
    </button>
  );
}

// CountryOptions loads the country list once via the geo SDK.
function CountryOptions() {
  const [countries, setCountries] = useState<{ id: string; code: string; name: LocaleMap }[]>([]);
  useEffect(() => {
    api.geo.listCountries().then((r) => setCountries((r.countries ?? []) as { id: string; code: string; name: LocaleMap }[])).catch(() => {});
  }, []);
  return (
    <>
      <option value="">— country (optional) —</option>
      {countries.map((c) => <option key={c.id} value={c.id}>{c.code} — {pickLabel(c.name) || c.code}</option>)}
    </>
  );
}
