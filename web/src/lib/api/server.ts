import "server-only";
import { auth } from "@/auth";
import { createOikumeneaClient, type OikumeneaClient } from "oikumenea-client";

/**
 * Server-side access to the go-oikumenea API for Server Components / actions, via the typed SDK
 * (D-ClientSDK) bound to the session bearer and the real API base URL. The unified façade reaches
 * every service, including the hermenea endpoints oikumenea proxies:
 *
 *   const ok = await oikumenea();
 *   const person = await ok.person.getPerson(id);   // typed
 *   const runs = await ok.hermenea.listRuns();        // hermenea, through oikumenea
 *   const page = await ok.request("GET", def.list.path, { query: search }); // generic (registry)
 *
 * The access token is read from the httpOnly session and never reaches the browser. API_BASE_URL
 * points at the service (https://localhost:8443 in dev — self-signed; set NODE_TLS_REJECT_UNAUTHORIZED=0
 * for dev, see web/.env.example).
 */
export const API_BASE_URL = process.env.API_BASE_URL ?? "https://localhost:8443";

export async function oikumenea(): Promise<OikumeneaClient> {
  const session = await auth();
  const token = session?.accessToken;
  return createOikumeneaClient({
    baseUrl: API_BASE_URL,
    token: token ?? undefined,
  });
}

/**
 * Low-level forward used ONLY by the BFF proxy route (app/api/oikumenea/[...path]): it attaches the
 * bearer and relays method/body/query verbatim. Components never call this — they use the SDK above.
 */
export async function apiForward(
  path: string,
  init: RequestInit & { search?: string } = {},
): Promise<Response> {
  const session = await auth();
  const token = session?.accessToken;
  const { search, headers, ...rest } = init;
  const url = `${API_BASE_URL}${path}${search ?? ""}`;
  const h = new Headers(headers);
  if (token) h.set("Authorization", `Bearer ${token}`);
  if (!h.has("Accept")) h.set("Accept", "application/json");
  return fetch(url, { ...rest, headers: h, cache: "no-store" });
}
