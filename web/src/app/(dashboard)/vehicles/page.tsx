"use client";

// Vehicle workspace (M26 / D-Vehicles). Browse/create vehicles and drill into one to see and record its
// registrations (the ownership+plate history) and the brand→manufacturer links. Brand/model/type and
// plate-number catalogs are managed here too. The plate region is a WOF geo_places region (placetype=
// region) resolved per country. A person's vehicles are surfaced on the person object view.

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { SearchSelect } from "@/components/SearchSelect";
import { PageHeader, Card, Table, Mono } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { newSuffix, slugify } from "@/lib/code";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Catalog = { id: string; code: string; name: LocaleMap };
type Brand = { id: string; code: string; name: LocaleMap; countryId?: string };
type Model = { id: string; brandId: string; code: string; name: LocaleMap; generation?: string };
type Vehicle = { id: string; typeId: string; typeLabel?: string; modelId?: string; modelLabel?: string; brandLabel?: string; vin?: string; color?: string; status: string };
type Registration = {
  id: string; vehicleId: string; ownerKind: string; ownerId: string; ownerLabel?: string;
  countryId: string; subdivisionId?: string; subdivisionLabel?: string; registrationNumber: string;
  status: string; effectiveFrom: string; effectiveTo?: string;
};
type Manufacturer = { id: string; brandId: string; companyId: string; companyLabel?: string; effectiveFrom?: string; effectiveTo?: string };
type Place = { id: string; name: string };

const label = (m: LocaleMap, fallback: string) => pickLabel(m) || fallback;

export default function VehiclesPage() {
  const [types, setTypes] = useState<Catalog[]>([]);
  const [brands, setBrands] = useState<Brand[]>([]);
  const [numberTypes, setNumberTypes] = useState<Catalog[]>([]);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [selected, setSelected] = useState<Vehicle | null>(null);
  const [err, setErr] = useState<unknown>(null);

  function reload() {
    api.vehicle.listVehicles(undefined, 100).then((r) => setVehicles((r.vehicles ?? []) as unknown as Vehicle[])).catch(setErr);
  }
  useEffect(() => {
    api.vehicle.listVehicleTypes().then((r) => setTypes((r.types ?? []) as unknown as Catalog[])).catch(() => {});
    api.vehicle.listBrands().then((r) => setBrands((r.brands ?? []) as unknown as Brand[])).catch(() => {});
    api.vehicle.listRegistrationNumberTypes().then((r) => setNumberTypes((r.numberTypes ?? []) as unknown as Catalog[])).catch(() => {});
    reload();
  }, []);

  return (
    <div>
      <PageHeader
        title={<T>Vehicles</T>}
        description={<T>A vehicle registry — brand/model/type taxonomy, the physical vehicle (VIN), the brand→manufacturer link, and the ownership+plate registration record. External reference data, independent of the deploying org's units.</T>}
      />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}

      <div className="grid gap-6 lg:grid-cols-2">
        <CreateVehicle types={types} brands={brands} onCreated={reload} setErr={setErr} />
        <CatalogsCard brands={brands} onChanged={() => api.vehicle.listBrands().then((r) => setBrands((r.brands ?? []) as unknown as Brand[]))} setErr={setErr} />
      </div>

      <Card className="mt-6">
        <h2 className="mb-2 text-sm font-semibold text-slate-900"><T>Registry</T></h2>
        <Table head={<><th className="th"><T>Vehicle</T></th><th className="th">VIN</th><th className="th"><T>Make</T></th><th className="th"><T>Status</T></th><th className="th"></th></>}>
          {vehicles.map((v) => (
            <tr key={v.id} className="border-t">
              <td className="py-1"><Link href={`/o/${v.id}`} className="text-indigo-600 hover:underline"><Mono>{v.id.slice(-8)}</Mono></Link></td>
              <td>{v.vin || "—"}</td>
              <td>{[v.brandLabel, v.modelLabel].filter(Boolean).join(" ") || v.typeLabel || "—"}</td>
              <td>{v.status}</td>
              <td><button className="text-xs text-indigo-600 hover:underline" onClick={() => setSelected(v)}><T>Open</T></button></td>
            </tr>
          ))}
        </Table>
      </Card>

      {selected ? (
        <VehicleDetail vehicle={selected} brands={brands} numberTypes={numberTypes} setErr={setErr} />
      ) : null}
    </div>
  );
}

function CreateVehicle({ types, brands, onCreated, setErr }: { types: Catalog[]; brands: Brand[]; onCreated: () => void; setErr: (e: unknown) => void }) {
  const [typeId, setTypeId] = useState("");
  const [brandId, setBrandId] = useState("");
  const [models, setModels] = useState<Model[]>([]);
  const [modelId, setModelId] = useState("");
  const [vin, setVin] = useState("");
  const [color, setColor] = useState("");

  useEffect(() => {
    if (!brandId) { setModels([]); setModelId(""); return; }
    api.vehicle.listModels(brandId).then((r) => setModels((r.models ?? []) as unknown as Model[])).catch(() => setModels([]));
  }, [brandId]);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!typeId) return;
    api.vehicle
      .createVehicle({ typeId, modelId: modelId || undefined, vin: vin || undefined, color: color || undefined })
      .then(() => { setVin(""); setColor(""); onCreated(); })
      .catch(setErr);
  }

  return (
    <Card>
      <h2 className="mb-2 text-sm font-semibold text-slate-900"><T>New vehicle</T></h2>
      <form onSubmit={submit} className="space-y-2 text-sm">
        <select className="input" value={typeId} onChange={(e) => setTypeId(e.target.value)} required>
          <option value="">— type —</option>
          {types.map((t) => <option key={t.id} value={t.id}>{label(t.name, t.code)}</option>)}
        </select>
        <select className="input" value={brandId} onChange={(e) => setBrandId(e.target.value)}>
          <option value="">— brand (optional) —</option>
          {brands.map((b) => <option key={b.id} value={b.id}>{label(b.name, b.code)}</option>)}
        </select>
        {brandId ? (
          <select className="input" value={modelId} onChange={(e) => setModelId(e.target.value)}>
            <option value="">— model (optional) —</option>
            {models.map((m) => <option key={m.id} value={m.id}>{label(m.name, m.code)}</option>)}
          </select>
        ) : null}
        <input className="input" placeholder="VIN (optional)" value={vin} onChange={(e) => setVin(e.target.value)} />
        <input className="input" placeholder="Color (optional)" value={color} onChange={(e) => setColor(e.target.value)} />
        <button className="btn-primary" type="submit"><T>Create</T></button>
      </form>
    </Card>
  );
}

function CatalogsCard({ brands, onChanged, setErr }: { brands: Brand[]; onChanged: () => void; setErr: (e: unknown) => void }) {
  const [brandName, setBrandName] = useState("");
  const [brandCountry, setBrandCountry] = useState("");
  const [modelBrand, setModelBrand] = useState("");
  const [modelName, setModelName] = useState("");

  function addBrand(e: React.FormEvent) {
    e.preventDefault();
    if (!brandName.trim()) return;
    api.vehicle
      .upsertBrand({ code: slugify(brandName) + newSuffix(), name: brandName, countryId: brandCountry || undefined })
      .then(() => { setBrandName(""); setBrandCountry(""); onChanged(); })
      .catch(setErr);
  }
  function addModel(e: React.FormEvent) {
    e.preventDefault();
    if (!modelBrand || !modelName.trim()) return;
    api.vehicle
      .upsertModel(modelBrand, { code: slugify(modelName) + newSuffix(), name: modelName })
      .then(() => { setModelName(""); })
      .catch(setErr);
  }

  return (
    <Card>
      <h2 className="mb-2 text-sm font-semibold text-slate-900"><T>Catalogs</T></h2>
      <form onSubmit={addBrand} className="mb-3 space-y-2 text-sm">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-400"><T>New brand</T></p>
        <input className="input" placeholder="Brand name" value={brandName} onChange={(e) => setBrandName(e.target.value)} />
        <CountrySelectControlled value={brandCountry} onChange={setBrandCountry} />
        <button className="btn-secondary" type="submit"><T>Add brand</T></button>
      </form>
      <form onSubmit={addModel} className="space-y-2 text-sm">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-400"><T>New model</T></p>
        <select className="input" value={modelBrand} onChange={(e) => setModelBrand(e.target.value)}>
          <option value="">— brand —</option>
          {brands.map((b) => <option key={b.id} value={b.id}>{label(b.name, b.code)}</option>)}
        </select>
        <input className="input" placeholder="Model name" value={modelName} onChange={(e) => setModelName(e.target.value)} />
        <button className="btn-secondary" type="submit"><T>Add model</T></button>
      </form>
    </Card>
  );
}

// CountrySelectControlled wraps the uncontrolled CountrySelect for a controlled value via a hidden
// re-mount keyed on the chosen value (the page only needs the id, kept in React state).
function CountrySelectControlled({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  return (
    <select className="input" value={value} onChange={(e) => onChange(e.target.value)}>
      <CountryOptions />
    </select>
  );
}

// CountryOptions loads the country list once via the geo SDK.
function CountryOptions() {
  const [countries, setCountries] = useState<{ id: string; code: string; name: string }[]>([]);
  useEffect(() => {
    api.geo.listCountries().then((r) => setCountries((r.countries ?? []) as { id: string; code: string; name: string }[])).catch(() => {});
  }, []);
  return (
    <>
      <option value="">— country (optional) —</option>
      {countries.map((c) => <option key={c.id} value={c.id}>{c.code} — {c.name}</option>)}
    </>
  );
}

function VehicleDetail({ vehicle, brands, numberTypes, setErr }: { vehicle: Vehicle; brands: Brand[]; numberTypes: Catalog[]; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [regs, setRegs] = useState<Registration[]>([]);
  const [mans, setMans] = useState<Manufacturer[]>([]);
  const [manBrand, setManBrand] = useState("");

  function reloadRegs() {
    api.vehicle.listRegistrations(vehicle.id).then((r) => setRegs((r.registrations ?? []) as unknown as Registration[])).catch(setErr);
  }
  useEffect(() => { reloadRegs(); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [vehicle.id]);

  function close(regId: string) {
    api.vehicle.closeRegistration(regId).then(reloadRegs).catch(setErr);
  }

  return (
    <Card className="mt-6">
      <h2 className="mb-2 text-sm font-semibold text-slate-900">
        <T>Vehicle</T> <Mono>{vehicle.id.slice(-8)}</Mono>
        {vehicle.vin ? ` · ${vehicle.vin}` : ""}
      </h2>

      <RegisterForm vehicleId={vehicle.id} numberTypes={numberTypes} onDone={reloadRegs} setErr={setErr} />

      <h3 className="mb-1 mt-4 text-xs font-semibold uppercase tracking-wide text-slate-400"><T>Registrations (ownership history)</T></h3>
      <ul className="space-y-1 text-sm">
        {regs.map((r) => (
          <li key={r.id} className="flex items-center gap-2">
            <span className="font-medium">{r.registrationNumber}</span>
            <span className="text-slate-500">{r.ownerKind === "company" ? (r.ownerLabel || r.ownerId.slice(-8)) : `person ${r.ownerId.slice(-8)}`}</span>
            {r.subdivisionLabel ? <span className="text-slate-400">· {r.subdivisionLabel}</span> : null}
            <span className="text-slate-400">· {r.status}</span>
            {r.status === "active" ? <button className="text-xs text-indigo-600 hover:underline" onClick={() => close(r.id)}><T>Close</T></button> : null}
          </li>
        ))}
        {regs.length === 0 ? <li className="text-slate-400"><T>No registrations.</T></li> : null}
      </ul>

      <h3 className="mb-1 mt-4 text-xs font-semibold uppercase tracking-wide text-slate-400"><T>Brand manufacturers</T></h3>
      <div className="flex items-center gap-2 text-sm">
        <select className="input max-w-xs" value={manBrand} onChange={(e) => { setManBrand(e.target.value); if (e.target.value) api.vehicle.listManufacturers(e.target.value).then((r) => setMans((r.manufacturers ?? []) as unknown as Manufacturer[])).catch(setErr); }}>
          <option value="">— brand —</option>
          {brands.map((b) => <option key={b.id} value={b.id}>{pickLabel(b.name) || b.code}</option>)}
        </select>
        {manBrand ? (
          <SearchSelect kind="company" placeholder={tr("Add manufacturer (company)…")} onChange={(companyId) => api.vehicle.addManufacturer(manBrand, { companyId }).then(() => api.vehicle.listManufacturers(manBrand).then((r) => setMans((r.manufacturers ?? []) as unknown as Manufacturer[]))).catch(setErr)} />
        ) : null}
      </div>
      <ul className="mt-1 space-y-0.5 text-sm">
        {mans.map((m) => <li key={m.id}>{m.companyLabel || m.companyId.slice(-8)}{m.effectiveFrom ? ` · ${m.effectiveFrom}` : ""}</li>)}
      </ul>
    </Card>
  );
}

function RegisterForm({ vehicleId, numberTypes, onDone, setErr }: { vehicleId: string; numberTypes: Catalog[]; onDone: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [ownerKind, setOwnerKind] = useState<"person" | "company">("person");
  const [ownerId, setOwnerId] = useState("");
  const [countryId, setCountryId] = useState("");
  const [places, setPlaces] = useState<Place[]>([]);
  const [subdivisionId, setSubdivisionId] = useState("");
  const [plate, setPlate] = useState("");
  const [numberTypeId, setNumberTypeId] = useState("");

  useEffect(() => {
    setSubdivisionId("");
    if (!countryId) { setPlaces([]); return; }
    api.geo.listPlaces(countryId, "region").then((r) => setPlaces((r.places ?? []) as unknown as Place[])).catch(() => setPlaces([]));
  }, [countryId]);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!ownerId || !countryId || !plate.trim()) return;
    api.vehicle
      .registerVehicle(vehicleId, { ownerKind, ownerId, countryId, subdivisionId: subdivisionId || undefined, registrationNumber: plate, numberTypeId: numberTypeId || undefined })
      .then(() => { setPlate(""); setOwnerId(""); onDone(); })
      .catch(setErr);
  }

  return (
    <form onSubmit={submit} className="grid gap-2 text-sm sm:grid-cols-2">
      <select className="input" value={ownerKind} onChange={(e) => { setOwnerKind(e.target.value as "person" | "company"); setOwnerId(""); }}>
        <option value="person">person</option>
        <option value="company">company</option>
      </select>
      <SearchSelect kind={ownerKind} key={ownerKind} placeholder={tr("Owner…")} onChange={setOwnerId} />
      <CountrySelectControlled value={countryId} onChange={setCountryId} />
      <select className="input" value={subdivisionId} onChange={(e) => setSubdivisionId(e.target.value)} disabled={!countryId}>
        <option value="">— region (optional) —</option>
        {places.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
      </select>
      <input className="input" placeholder={tr("Plate number")} value={plate} onChange={(e) => setPlate(e.target.value)} />
      <select className="input" value={numberTypeId} onChange={(e) => setNumberTypeId(e.target.value)}>
        <option value="">— plate type (optional) —</option>
        {numberTypes.map((n) => <option key={n.id} value={n.id}>{pickLabel(n.name) || n.code}</option>)}
      </select>
      <button className="btn-primary sm:col-span-2" type="submit"><T>Register / transfer</T></button>
    </form>
  );
}
