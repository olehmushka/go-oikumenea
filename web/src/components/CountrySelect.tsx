"use client";

// Country picker backed by the geo registry. Countries are RID-keyed (F-014): the option VALUE is the
// country's RID (what person/document/rank store), the label is "CODE — Name". Renders a plain
// <select name=...> so it drops into the existing FormData-based forms unchanged. Reads through the BFF
// proxy (GET /geo/v1/countries; the proxy injects the bearer).

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

export type Country = { id: string; code: string; name: LocaleMap; status: string };

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
  value,
  onChange,
  required,
  includeEmpty = true,
}: {
  name: string;
  defaultValue?: string;
  // Controlled mode: pass both `value` and `onChange` (e.g. so a coordinate lookup can prefill the
  // country). Omit them to keep the uncontrolled `defaultValue` behaviour used elsewhere.
  value?: string;
  onChange?: (id: string) => void;
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

  const controlled = value !== undefined && onChange !== undefined;

  return (
    <select
      name={name}
      className="input"
      required={required}
      {...(controlled
        ? { value, onChange: (e) => onChange!(e.target.value) }
        : { defaultValue: defaultValue ?? "" })}
    >
      {includeEmpty ? <option value="">—</option> : null}
      {countries.map((c) => (
        <option key={c.id} value={c.id}>
          {c.code} — {pickLabel(c.name) || c.code}
        </option>
      ))}
    </select>
  );
}
