"use client";

import { createOikumeneaClient } from "oikumenea-client";
import { authAwareFetch } from "./unauthorized";

/**
 * The single client for Client Components — the unified go-oikumenea SDK (D-ClientSDK), bound to the
 * BFF proxy (/api/oikumenea), NOT the API directly. The proxy injects the bearer server-side, so no
 * token lives in the browser. Reaches every service (incl. the hermenea endpoints oikumenea proxies)
 * via typed methods (`api.person.getPerson(id)`) or the generic escape hatch (`api.request(...)`) for
 * the data-driven ontology registry. Types come from the same Conjure contract as the server, so calls
 * cannot drift (D-WebUI).
 *
 * The custom fetch intercepts a 401 (dead session) and instantly signs the user out to /login instead
 * of letting a stale-session error surface in the UI (see ./unauthorized).
 */
export const api = createOikumeneaClient({ baseUrl: "/api/oikumenea", fetch: authAwareFetch });
