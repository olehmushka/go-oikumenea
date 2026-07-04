"use client";

// Reusable create-location form (M19 / D-Location). Extracted from the /locations page so it can be
// dropped into a modal (see SearchSelect's inline "＋ create location"). Self-fetches location types,
// converts a coordinate in any supported format to WGS84 server-side, and calls onCreated with the new
// location. Callers own the surrounding chrome (page success card, modal, …).

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { CountrySelect } from "@/components/CountrySelect";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

export type CoordinateInput = {
  format: string;
  latitude?: number;
  longitude?: number;
  mgrs?: string;
  zone?: number;
  hemisphere?: string;
  easting?: number;
  northing?: number;
  grid?: string;
};
export type Location = {
  id: string;
  latitude: number;
  longitude: number;
  mgrs?: string;
  sourceCoordinate?: CoordinateInput;
  countryId: string;
  locality?: string;
  typeName?: LocaleMap;
};
type LocationType = { id: string; code: string; name: LocaleMap; status: string };

const FORMATS: { value: string; label: string }[] = [
  { value: "latlon", label: "WGS84 lat/lon" },
  { value: "mgrs", label: "MGRS" },
  { value: "utm", label: "UTM" },
  { value: "sk42", label: "СК-42 (Gauss-Krüger)" },
  { value: "sk42grid", label: "СК-42 grid" },
];

function buildCoordinate(format: string, f: FormData): CoordinateInput {
  const num = (k: string) => {
    const v = String(f.get(k) || "").trim();
    return v === "" ? undefined : Number(v);
  };
  const str = (k: string) => {
    const v = String(f.get(k) || "").trim();
    return v === "" ? undefined : v;
  };
  switch (format) {
    case "mgrs":
      return { format, mgrs: str("mgrs") };
    case "utm":
      return { format, zone: num("zone"), hemisphere: str("hemisphere") ?? "N", easting: num("easting"), northing: num("northing") };
    case "sk42":
      return { format, zone: num("zone"), easting: num("easting"), northing: num("northing") };
    case "sk42grid":
      return { format, grid: str("grid") };
    default:
      return { format: "latlon", latitude: num("latitude"), longitude: num("longitude") };
  }
}

export function LocationForm({ onCreated, submitLabel }: { onCreated: (loc: Location) => void; submitLabel?: React.ReactNode }) {
  const tr = useTg();
  const [types, setTypes] = useState<LocationType[]>([]);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [format, setFormat] = useState("latlon");
  // Prefilled-from-coordinate fields (controlled so the lookup can fill them); see onLatLonBlur.
  const [countryId, setCountryId] = useState("");
  const [locality, setLocality] = useState("");
  const [adminArea1, setAdminArea1] = useState("");

  useEffect(() => {
    api.location.listLocationTypes()
      .then((r) => setTypes(r.locationTypes ?? []))
      .catch(() => setTypes([]));
  }, []);

  // onLatLonBlur reverse-geocodes the entered WGS84 coordinate and prefills Country / Locality / Admin
  // area — but only fields the user has left empty (manual edits are never overwritten). Best-effort:
  // any error is swallowed so it never blocks creation. It fires only when BOTH latitude and longitude
  // are actually present: Number("") is 0 (finite), so a raw empty-string check is required to avoid
  // prefilling from a half-entered coordinate (e.g. latitude typed, longitude still blank).
  async function onLatLonBlur(e: React.FocusEvent<HTMLInputElement>) {
    const fForm = e.currentTarget.form;
    if (!fForm) return;
    const f = new FormData(fForm);
    const latRaw = String(f.get("latitude") || "").trim();
    const lngRaw = String(f.get("longitude") || "").trim();
    if (latRaw === "" || lngRaw === "") return;
    const lat = Number(latRaw);
    const lng = Number(lngRaw);
    if (!Number.isFinite(lat) || !Number.isFinite(lng)) return;
    try {
      const res = await api.geo.resolveCoordinate(lat, lng);
      if (res.country) setCountryId((prev) => prev || res.country!.id);
      const place = res.place;
      if (place) {
        if (place.placetype === "locality") setLocality((prev) => prev || place.name);
        else setAdminArea1((prev) => prev || place.name);
      }
    } catch {
      // best-effort prefill
    }
  }

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    setBusy(true);
    setErr(null);
    const f = new FormData(form);
    const str = (k: string) => {
      const v = String(f.get(k) || "").trim();
      return v === "" ? undefined : v;
    };
    const body = {
      coordinate: buildCoordinate(format, f),
      countryId: String(f.get("countryId") || "").trim(),
      typeId: str("typeId"),
      locality: str("locality"),
      street: str("street"),
      houseNumber: str("houseNumber"),
      postalCode: str("postalCode"),
      adminArea1: str("adminArea1"),
      rawAddress: str("rawAddress"),
    };
    try {
      const loc = await api.location.createLocation(body as never);
      // Clear the form for the next entry: reset the uncontrolled inputs (coordinate, street, …) and
      // the controlled prefill fields, so the next coordinate re-triggers the country/locality lookup.
      form.reset();
      setCountryId("");
      setLocality("");
      setAdminArea1("");
      onCreated(loc as unknown as Location);
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-3">
      {err ? <div><ErrorBox error={err} /></div> : null}
      <div>
        <label className="label"><T>Coordinate format</T></label>
        <select className="input" value={format} onChange={(e) => setFormat(e.target.value)}>
          {FORMATS.map((f) => (
            <option key={f.value} value={f.value}>{f.label}</option>
          ))}
        </select>
      </div>

      <CoordinateFields format={format} onLatLonBlur={onLatLonBlur} />

      <div>
        <label className="label"><T>Country *</T></label>
        <CountrySelect name="countryId" required value={countryId} onChange={setCountryId} />
      </div>
      <div>
        <label className="label"><T>Type</T></label>
        <select name="typeId" className="input" defaultValue="">
          <option value="">—</option>
          {types.map((t) => (
            <option key={t.id} value={t.id}>
              {pickLabel(t.name) || t.code}
            </option>
          ))}
        </select>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label"><T>Locality</T></label>
          <input name="locality" className="input" placeholder={tr("Kyiv")} value={locality} onChange={(e) => setLocality(e.target.value)} />
        </div>
        <div>
          <label className="label"><T>Admin area</T></label>
          <input name="adminArea1" className="input" placeholder={tr("Kyiv City")} value={adminArea1} onChange={(e) => setAdminArea1(e.target.value)} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label"><T>Street</T></label>
          <input name="street" className="input" placeholder={tr("Maidan Nezalezhnosti")} />
        </div>
        <div>
          <label className="label"><T>House no.</T></label>
          <input name="houseNumber" className="input" placeholder="1" />
        </div>
      </div>
      <div>
        <label className="label"><T>Postal code</T></label>
        <input name="postalCode" className="input" placeholder="01001" />
      </div>
      <button type="submit" className="btn" disabled={busy}>
        {busy ? <T>Creating…</T> : submitLabel ?? <T>Create location</T>}
      </button>
    </form>
  );
}

// CoordinateFields swaps the inputs for the chosen format. All inputs are plain (uncontrolled) and read
// from FormData on submit; only the rendered set changes.
function CoordinateFields({ format, onLatLonBlur }: { format: string; onLatLonBlur?: (e: React.FocusEvent<HTMLInputElement>) => void }) {
  switch (format) {
    case "mgrs":
      return (
        <div>
          <label className="label"><T>MGRS *</T></label>
          <input name="mgrs" required className="input" placeholder="36UUA2418291607" />
        </div>
      );
    case "utm":
      return (
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="label"><T>Zone *</T></label>
            <input name="zone" required className="input" placeholder="36" inputMode="numeric" />
          </div>
          <div>
            <label className="label"><T>Hemisphere</T></label>
            <select name="hemisphere" className="input" defaultValue="N">
              <option value="N">N</option>
              <option value="S">S</option>
            </select>
          </div>
          <div>
            <label className="label"><T>Easting *</T></label>
            <input name="easting" required className="input" placeholder="324182" inputMode="decimal" />
          </div>
          <div>
            <label className="label"><T>Northing *</T></label>
            <input name="northing" required className="input" placeholder="5591607" inputMode="decimal" />
          </div>
        </div>
      );
    case "sk42":
      return (
        <div className="grid grid-cols-3 gap-3">
          <div>
            <label className="label"><T>Zone *</T></label>
            <input name="zone" required className="input" placeholder="6" inputMode="numeric" />
          </div>
          <div>
            <label className="label"><T>Easting (Y) *</T></label>
            <input name="easting" required className="input" placeholder="388000" inputMode="decimal" />
          </div>
          <div>
            <label className="label"><T>Northing (X) *</T></label>
            <input name="northing" required className="input" placeholder="5590000" inputMode="decimal" />
          </div>
        </div>
      );
    case "sk42grid":
      return (
        <div>
          <label className="label"><T>СК-42 grid *</T></label>
          <input name="grid" required className="input" placeholder="6 5590000 388000" />
          <p className="mt-1 text-xs text-slate-400"><T>Full numeric reference: zone northing easting (metres).</T></p>
        </div>
      );
    default:
      return (
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="label"><T>Latitude *</T></label>
            <input name="latitude" required className="input" placeholder="50.4501" inputMode="decimal" onBlur={onLatLonBlur} />
          </div>
          <div>
            <label className="label"><T>Longitude *</T></label>
            <input name="longitude" required className="input" placeholder="30.5234" inputMode="decimal" onBlur={onLatLonBlur} />
          </div>
        </div>
      );
  }
}
