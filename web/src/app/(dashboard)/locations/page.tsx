"use client";

// Locations workspace (M19 / D-Location). The browse/create surface for the shared place entity:
// create a location from a coordinate in any supported format (the server converts it to WGS84 and
// derives the MGRS in the application), and run a radius (ST_DWithin) search. A location has no global
// list (a spatial window is required), so this page is its home; the per-object view lives at /o/<id>.

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { CountrySelect } from "@/components/CountrySelect";
import { PageHeader, Card, Table, Mono } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type CoordinateInput = {
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
type Location = {
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

export default function LocationsPage() {
  const [types, setTypes] = useState<LocationType[]>([]);
  useEffect(() => {
    api.location.listLocationTypes()
      .then((r) => setTypes(r.locationTypes ?? []))
      .catch(() => setTypes([]));
  }, []);

  return (
    <div>
      <PageHeader
        title={<T>Locations</T>}
        description={<T>The shared place entity (D-Location): a precise coordinate with a structured address. The coordinate can be entered in several formats (lat/lon, MGRS, UTM, СК-42); the server converts it to WGS84 and derives the MGRS in the application.</T>}
      />
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <CreateLocation types={types} />
        <RadiusSearch />
      </div>
    </div>
  );
}

function CreateLocation({ types }: { types: LocationType[] }) {
  const tr = useTg();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<Location | null>(null);
  const [format, setFormat] = useState("latlon");

  function buildCoordinate(f: FormData): CoordinateInput {
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

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    setCreated(null);
    const f = new FormData(e.currentTarget);
    const str = (k: string) => {
      const v = String(f.get(k) || "").trim();
      return v === "" ? undefined : v;
    };
    const body = {
      coordinate: buildCoordinate(f),
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
      setCreated(loc as unknown as Location);
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Create a location</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="space-y-3">
        <div>
          <label className="label"><T>Coordinate format</T></label>
          <select className="input" value={format} onChange={(e) => setFormat(e.target.value)}>
            {FORMATS.map((f) => (
              <option key={f.value} value={f.value}>{f.label}</option>
            ))}
          </select>
        </div>

        <CoordinateFields format={format} />

        <div>
          <label className="label"><T>Country *</T></label>
          <CountrySelect name="countryId" required />
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
            <input name="locality" className="input" placeholder={tr("Kyiv")} />
          </div>
          <div>
            <label className="label"><T>Admin area</T></label>
            <input name="adminArea1" className="input" placeholder={tr("Kyiv City")} />
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
          {busy ? <T>Creating…</T> : <T>Create location</T>}
        </button>
      </form>

      {created ? (
        <div className="mt-4 rounded-md border border-green-200 bg-green-50 p-3 text-sm">
          <div className="mb-2 font-medium text-green-800">
            <T>Created —</T> <Link href={`/o/${created.id}`} className="underline"><T>open</T></Link>
          </div>
          <dl className="grid grid-cols-[8rem_1fr] gap-y-1">
            <dt className="text-slate-500">MGRS</dt>
            <dd><Mono>{created.mgrs ?? "—"}</Mono></dd>
            <dt className="text-slate-500">WGS84</dt>
            <dd><Mono>{created.latitude}, {created.longitude}</Mono></dd>
            <dt className="text-slate-500"><T>Source</T></dt>
            <dd><Mono>{created.sourceCoordinate?.format ?? "—"}</Mono></dd>
          </dl>
        </div>
      ) : null}
    </Card>
  );
}

// CoordinateFields swaps the inputs for the chosen format. All inputs are plain (uncontrolled) and read
// from FormData on submit; only the rendered set changes.
function CoordinateFields({ format }: { format: string }) {
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
            <input name="latitude" required className="input" placeholder="50.4501" inputMode="decimal" />
          </div>
          <div>
            <label className="label"><T>Longitude *</T></label>
            <input name="longitude" required className="input" placeholder="30.5234" inputMode="decimal" />
          </div>
        </div>
      );
  }
}

function RadiusSearch() {
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [rows, setRows] = useState<Location[] | null>(null);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    const f = new FormData(e.currentTarget);
    const q = new URLSearchParams({
      lat: String(f.get("lat") || "").trim(),
      lng: String(f.get("lng") || "").trim(),
      radiusM: String(f.get("radiusM") || "").trim(),
      pageSize: "50",
    });
    try {
      const res = await api.request<{ locations: Location[] }>("GET", `/location/v1/locations?${q.toString()}`);
      setRows(res.locations ?? []);
    } catch (e) {
      setErr(e);
      setRows(null);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Radius search</T></h2>
      {err ? <div className="mb-3"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="mb-4 grid grid-cols-3 gap-3">
        <div>
          <label className="label"><T>Lat</T></label>
          <input name="lat" required className="input" placeholder="50.4501" inputMode="decimal" />
        </div>
        <div>
          <label className="label"><T>Lng</T></label>
          <input name="lng" required className="input" placeholder="30.5234" inputMode="decimal" />
        </div>
        <div>
          <label className="label"><T>Radius (m)</T></label>
          <input name="radiusM" required className="input" placeholder="5000" inputMode="numeric" />
        </div>
        <button type="submit" className="btn col-span-3" disabled={busy}>
          {busy ? <T>Searching…</T> : <T>Search nearby</T>}
        </button>
      </form>

      {rows == null ? (
        <p className="text-sm text-slate-400"><T>Enter a centre point and radius to find nearby locations.</T></p>
      ) : rows.length === 0 ? (
        <p className="text-sm text-slate-400"><T>No locations within the radius.</T></p>
      ) : (
        <Table
          head={
            <>
              <th className="th"><T>MGRS</T></th>
              <th className="th"><T>Locality</T></th>
              <th className="th text-right"><T>Lat</T></th>
              <th className="th text-right"><T>Lng</T></th>
            </>
          }
        >
          {rows.map((l) => (
            <tr key={l.id} className="hover:bg-slate-50">
              <td className="td">
                <Link href={`/o/${l.id}`} className="text-indigo-600 hover:underline">
                  <Mono>{l.mgrs ?? l.id.slice(-6)}</Mono>
                </Link>
              </td>
              <td className="td">{l.locality ?? "—"}</td>
              <td className="td text-right"><Mono>{l.latitude}</Mono></td>
              <td className="td text-right"><Mono>{l.longitude}</Mono></td>
            </tr>
          ))}
        </Table>
      )}
    </Card>
  );
}
