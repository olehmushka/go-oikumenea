"use client";

// Country picker backed by the geo registry. Countries are RID-keyed (F-014): the option VALUE is the
// country's RID (what person/document/rank store), the label is "CODE — Name". Renders a plain
// <select name=...> so it drops into the existing FormData-based forms unchanged. Reads through the BFF
// proxy (GET /geo/v1/countries; the proxy injects the bearer).

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";

export type Country = { id: string; code: string; name: string; status: string };

let cache: Country[] | null = null;

// useCountryMap returns a RID -> "CODE" lookup so display sites can render the ISO code for a stored
// country RID (countries are RID-keyed; the wire carries the RID). Returns "" for an unknown/loading id.
export function useCountryMap(): (id: string | undefined) => string {
  const [map, setMap] = useState<Record<string, string>>(() =>
    cache ? Object.fromEntries(cache.map((c) => [c.id, c.code])) : {},
  );
  useEffect(() => {
    if (cache) return;
    api.geo
      .listCountries()
      .then((r) => {
        cache = r.countries ?? [];
        setMap(Object.fromEntries(cache.map((c) => [c.id, c.code])));
      })
      .catch(() => {});
  }, []);
  return (id) => (id ? map[id] ?? "" : "");
}

export function CountrySelect({
  name,
  defaultValue,
  required,
  includeEmpty = true,
}: {
  name: string;
  defaultValue?: string;
  required?: boolean;
  includeEmpty?: boolean;
}) {
  const [countries, setCountries] = useState<Country[]>(cache ?? []);

  useEffect(() => {
    if (cache) return;
    api.geo
      .listCountries()
      .then((r) => {
        cache = r.countries ?? [];
        setCountries(cache);
      })
      .catch(() => setCountries([]));
  }, []);

  return (
    <select name={name} className="input" defaultValue={defaultValue ?? ""} required={required}>
      {includeEmpty ? <option value="">—</option> : null}
      {countries.map((c) => (
        <option key={c.id} value={c.id}>
          {c.code} — {c.name}
        </option>
      ))}
    </select>
  );
}
