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
 * The access token is read from the httpOnly session and never reaches the browser.
 *
 * This module runs inside **console-bff** — the facade (M52 / D-HeadlessTopology). Server Components
 * reach oikumenea directly from here; the browser reaches it through the BFF proxy route. Both attach
 * the same session bearer and both originate inside this process, on the internal network — oikumenea
 * publishes no host port in the packaged topology.
 *
 * API_BASE_URL is REQUIRED and has no default: in the compose topology it is the internal address
 * (https://app:8443), and for host development it is the locally-run binary (https://localhost:8443 —
 * self-signed, so set NODE_TLS_REJECT_UNAUTHORIZED=0; see web/.env.example). A default would silently
 * point a misconfigured facade at a host port that no longer exists, so fail fast instead.
 */
function requireApiBaseUrl(): string {
  const raw = process.env.API_BASE_URL?.trim();
  if (!raw) {
    throw new Error(
      "API_BASE_URL is not set. console-bff must be told where oikumenea listens " +
        "(compose: https://app:8443 — host dev: https://localhost:8443). See web/.env.example.",
    );
  }
  return raw.replace(/\/+$/, "");
}

export const API_BASE_URL = requireApiBaseUrl();

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
