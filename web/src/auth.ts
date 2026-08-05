import NextAuth, { type DefaultSession } from "next-auth";
import { consoleProviders, providerById, type ForwardedToken } from "@/lib/auth/providers";

/**
 * Auth.js (NextAuth v5) configuration — the IdP seam for the optional admin console.
 *
 * Flow (D-WebUI): browser → IdP Authorization-Code → Auth.js exchanges the code SERVER-SIDE and keeps
 * the tokens in an httpOnly JWT session. The browser never receives a token; the BFF proxy
 * (app/api/oikumenea/[...path]) reads it from the session and attaches `Authorization: Bearer` when
 * forwarding to the go-oikumenea API.
 *
 * The offered providers come from `lib/auth/providers` and are ENV-DRIVEN (D-MultiIdPExamples), so
 * this file no longer knows about any particular IdP. What it does own is the consequence of being
 * multi-IdP: the forwarded token and the refresh endpoint are now per-provider, because different
 * IdPs put the API audience on different tokens (Keycloak: the access token, via its realm audience
 * mapper; public OIDC providers: the ID token, whose `aud` is this console's client id). L-AuthzOnly
 * is unchanged either way — the service still only validates a token issued elsewhere.
 */

declare module "next-auth" {
  interface Session {
    accessToken?: string;
    error?: "RefreshTokenError";
    user: {
      id?: string;
    } & DefaultSession["user"];
  }
}

// The token fields we persist (Auth.js's JWT is an open record; we read/write these keys).
interface AppToken {
  /** The bearer the BFF forwards — already the RIGHT token for the provider that issued it. */
  accessToken?: string;
  refreshToken?: string;
  expiresAt?: number; // epoch seconds
  /** Which registry provider signed this session in; drives refresh and token selection. */
  providerId?: string;
  error?: "RefreshTokenError";
}

/** Pick the token the service can actually verify for this provider. */
function forwardedToken(
  account: { access_token?: string | null; id_token?: string | null },
  which: ForwardedToken,
): string | undefined {
  const picked = which === "id_token" ? account.id_token : account.access_token;
  return picked ?? undefined;
}

/**
 * Run the refresh_token grant against the provider's own token endpoint, discovered from its issuer.
 * Discovery (rather than a hardcoded URL) is what keeps this provider-agnostic.
 */
async function refreshAccessToken(providerId: string, refreshToken: string) {
  const provider = providerById[providerId];
  if (!provider) throw new Error(`unknown provider ${providerId}`);

  const wellKnown = await fetch(`${provider.issuer}/.well-known/openid-configuration`).then((r) =>
    r.json(),
  );
  const tokenEndpoint = wellKnown.token_endpoint as string;
  if (!tokenEndpoint) throw new Error(`no token_endpoint for ${providerId}`);

  const res = await fetch(tokenEndpoint, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "refresh_token",
      client_id: process.env[`${provider.envPrefix}_ID`]!,
      client_secret: process.env[`${provider.envPrefix}_SECRET`]!,
      refresh_token: refreshToken,
    }),
  });
  const tokens = await res.json();
  if (!res.ok) throw tokens;
  return tokens as {
    access_token?: string;
    id_token?: string;
    expires_in: number;
    refresh_token?: string;
  };
}

// Auth.js requires a secret. In development, fall back to a fixed insecure value so the console
// boots without a .env file; in production a real AUTH_SECRET is mandatory.
const AUTH_SECRET =
  process.env.AUTH_SECRET ??
  (process.env.NODE_ENV !== "production"
    ? "dev-insecure-auth-secret-change-me-0000000000="
    : undefined);

export const { handlers, signIn, signOut, auth } = NextAuth({
  trustHost: true,
  secret: AUTH_SECRET,
  providers: consoleProviders.map((p) => p.factory()),
  callbacks: {
    async jwt({ token, account }) {
      const t = token as AppToken & Record<string, unknown>;

      // Initial sign-in: persist the provider and ITS correct bearer onto the (server-only) JWT.
      if (account) {
        const provider = providerById[account.provider];
        t.providerId = account.provider;
        t.accessToken = provider
          ? forwardedToken(account, provider.forward)
          : (account.access_token ?? undefined);
        t.refreshToken = account.refresh_token ?? undefined;
        t.expiresAt = account.expires_at ?? undefined;
        return token;
      }

      // Still valid (60s safety margin): reuse.
      if (t.expiresAt && Date.now() < (t.expiresAt - 60) * 1000) {
        return token;
      }

      // Expired: refresh, or flag the session so the UI can re-authenticate. Providers that never
      // issued a refresh_token (Google without offline access, say) simply fall through to re-login.
      if (!t.refreshToken || !t.providerId) return token;
      try {
        const provider = providerById[t.providerId];
        const refreshed = await refreshAccessToken(t.providerId, t.refreshToken);
        const next = forwardedToken(refreshed, provider?.forward ?? "access_token");
        // A refresh that does not return the token shape we forward leaves the session unusable;
        // flag it rather than silently pinning a stale or wrong-typed bearer.
        if (!next) throw new Error(`refresh returned no ${provider?.forward} for ${t.providerId}`);
        t.accessToken = next;
        t.expiresAt = Math.floor(Date.now() / 1000) + refreshed.expires_in;
        if (refreshed.refresh_token) t.refreshToken = refreshed.refresh_token;
        delete t.error;
      } catch {
        t.error = "RefreshTokenError";
      }
      return token;
    },
    async session({ session, token }) {
      // Expose ONLY what the browser needs. The access token is forwarded by the BFF;
      // it lives here so server code (auth()) can read it, not so the client can.
      const t = token as AppToken;
      session.accessToken = t.accessToken;
      session.error = t.error;
      return session;
    },
  },
  pages: {
    signIn: "/login",
  },
});
