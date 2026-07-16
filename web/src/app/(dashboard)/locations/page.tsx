"use client";

// Locations workspace (M19 / D-Location). The browse/create surface for the shared place entity:
// create a location from a coordinate in any supported format (the server converts it to WGS84 and
// derives the MGRS in the application), and run a radius (ST_DWithin) search. A location has no global
// list (a spatial window is required), so this page is its home; the per-object view lives at /o/<id>.

import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api/client";
import { LocationForm, type Location } from "@/components/LocationForm";
import { PageHeader, Card, Table, Mono } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";

export default function LocationsPage() {
  return (
    <div>
      <PageHeader
        title={<T>Locations</T>}
        description={<T>The shared place entity (D-Location): a precise coordinate with a structured address. The coordinate can be entered in several formats (lat/lon, MGRS, UTM, СК-42); the server converts it to WGS84 and derives the MGRS in the application.</T>}
      />
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <CreateLocation />
        <RadiusSearch />
      </div>
    </div>
  );
}

function CreateLocation() {
  const [created, setCreated] = useState<Location | null>(null);

  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-700"><T>Create a location</T></h2>
      <LocationForm onCreated={setCreated} />

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

function RadiusSearch() {
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [rows, setRows] = useState<Location[] | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  async function onDelete(id: string) {
    if (!window.confirm("Delete this location? This fails if the location is still in use.")) return;
    setDeleting(id);
    setErr(null);
    try {
      await api.location.deleteLocation(id);
      setRows((prev) => (prev ? prev.filter((r) => r.id !== id) : prev));
    } catch (e) {
      setErr(e);
    } finally {
      setDeleting(null);
    }
  }

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
              <th className="th"></th>
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
              <td className="td text-right">
                <button
                  type="button"
                  className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
                  disabled={deleting === l.id}
                  onClick={() => onDelete(l.id)}
                >
                  {deleting === l.id ? <T>Deleting…</T> : <T>Delete</T>}
                </button>
              </td>
            </tr>
          ))}
        </Table>
      )}
    </Card>
  );
}
