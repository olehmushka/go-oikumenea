"use client";

// useCatalog fetches a system-wide reference list (email/phone types, platforms, document types,
// personal-code schemes, relation types) through the BFF proxy and caches it module-wide, keyed by
// path. Generalizes the cache pattern in CountrySelect so each "add" widget loads its own catalog
// once — instead of the person detail page eagerly fetching every catalog server-side and blocking
// the initial render. In-flight requests are de-duped so mounting several widgets that share a path
// triggers a single fetch.

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";

const cache = new Map<string, unknown[]>();
const inflight = new Map<string, Promise<unknown[]>>();

export function useCatalog<T = unknown>(path: string): T[] {
  const [data, setData] = useState<T[]>(() => (cache.get(path) as T[]) ?? []);

  useEffect(() => {
    if (cache.has(path)) {
      setData(cache.get(path) as T[]);
      return;
    }
    let active = true;
    let p = inflight.get(path);
    if (!p) {
      p = api.request<T[]>("GET", path)
        .then((r) => {
          const arr = (r ?? []) as unknown[];
          cache.set(path, arr);
          inflight.delete(path);
          return arr;
        })
        .catch(() => {
          inflight.delete(path);
          return [] as unknown[];
        });
      inflight.set(path, p);
    }
    p.then((arr) => {
      if (active) setData(arr as T[]);
    });
    return () => {
      active = false;
    };
  }, [path]);

  return data;
}
