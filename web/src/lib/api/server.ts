import "server-only";
import { auth } from "@/auth";
import { ApiError } from "./errors";

export { ApiError };

/**
 * Server-side access to the go-oikumenea API. Used by Server Components (SSR reads) and by
 * the BFF proxy (client mutations). Reads the access token from the httpOnly session and
 * attaches it as a bearer; the token never reaches the browser.
 *
 * API_BASE_URL points at the service (https://localhost:8443 in dev — self-signed; set
 * NODE_TLS_REJECT_UNAUTHORIZED=0 for dev, see web/.env.example).
 */
export const API_BASE_URL = process.env.API_BASE_URL ?? "https://localhost:8443";

/** Low-level forward: attaches the bearer and relays method/body/query verbatim. */
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

/** Typed JSON GET for Server Components. Throws ApiError on non-2xx. */
export async function apiGet<T>(path: string, search?: string): Promise<T> {
  const res = await apiForward(path, { method: "GET", search });
  const body = await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, body);
  return body as T;
}

/** Typed JSON mutation (POST/PUT/PATCH/DELETE) for Server Components / actions. */
export async function apiSend<T>(
  method: "POST" | "PUT" | "PATCH" | "DELETE",
  path: string,
  body?: unknown,
): Promise<T> {
  const res = await apiForward(path, {
    method,
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const out = res.status === 204 ? null : await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, out);
  return out as T;
}
