"use client";

import createClient from "openapi-fetch";
import type { paths } from "./schema";

/**
 * Typed browser client for Client Components. Its baseUrl is the BFF proxy (/api/oikumenea),
 * NOT the API directly — the proxy injects the bearer server-side. Types come from the
 * generated schema, so calls cannot drift from the contract (D-WebUI).
 */
export const api = createClient<paths>({ baseUrl: "/api/oikumenea" });

/** Untyped escape hatch for client mutations; returns parsed JSON or throws the error body. */
export async function mutate<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const res = await fetch(`/api/oikumenea${path}`, {
    method,
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const out = res.status === 204 ? null : await res.json().catch(() => null);
  if (!res.ok) throw out ?? new Error(`Request failed (${res.status})`);
  return out as T;
}
