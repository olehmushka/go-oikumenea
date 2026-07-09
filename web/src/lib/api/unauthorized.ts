"use client";

import { signOut } from "next-auth/react";

/**
 * Global 401 handler for browser SDK calls. A 401 from the BFF proxy means the session's bearer is
 * missing/expired/invalid (the go-oikumenea auth middleware 401s before routing) — the session is dead,
 * so there is nothing to show the user. Instead of surfacing an ErrorNotice, we clear the stale Auth.js
 * session and bounce to /login (which restarts the Keycloak flow) — an instant logout.
 *
 * Guarded so a burst of concurrent 401s (a page fires several reads at once) triggers exactly one
 * sign-out/redirect, not a stampede.
 */
let loggingOut = false;

export function handleUnauthorized(): void {
  if (loggingOut || typeof window === "undefined") return;
  loggingOut = true;
  // Clears the httpOnly session cookie server-side, then hard-navigates to /login.
  void signOut({ redirectTo: "/login" });
}

/**
 * A fetch wrapper that forwards to the platform fetch, then fires {@link handleUnauthorized} on a 401.
 * The response is still returned so the SDK throws its normal ConjureError — but the redirect is
 * already in flight, so the transient error never lingers on screen.
 */
export const authAwareFetch: typeof fetch = async (input, init) => {
  const res = await fetch(input, init);
  if (res.status === 401) handleUnauthorized();
  return res;
};
