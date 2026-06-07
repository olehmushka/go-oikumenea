import NextAuth, { type DefaultSession } from "next-auth";
import Keycloak from "next-auth/providers/keycloak";

/**
 * Auth.js (NextAuth v5) configuration — the IdP seam for the optional admin console.
 *
 * Flow (D-WebUI): browser → Keycloak Authorization-Code → Auth.js exchanges the code
 * SERVER-SIDE and keeps the access/refresh token in an httpOnly JWT session. The browser
 * never receives a token; the BFF proxy (app/api/oikumenea/[...path]) reads the token from
 * the session and attaches `Authorization: Bearer` when forwarding to the go-oikumenea API.
 *
 * Provider config is read from AUTH_KEYCLOAK_ID / AUTH_KEYCLOAK_SECRET / AUTH_KEYCLOAK_ISSUER
 * (Auth.js conventions). The access token carries `aud: oikumenea` (realm audience mapper),
 * so the service validates it with its normal idp.issuers[] rules — L-AuthzOnly is unchanged.
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
  accessToken?: string;
  refreshToken?: string;
  expiresAt?: number; // epoch seconds
  error?: "RefreshTokenError";
}

const ISSUER = process.env.AUTH_KEYCLOAK_ISSUER!;

async function refreshAccessToken(refreshToken: string) {
  // Discover the token endpoint, then run the refresh_token grant against Keycloak.
  const wellKnown = await fetch(`${ISSUER}/.well-known/openid-configuration`).then((r) =>
    r.json(),
  );
  const res = await fetch(wellKnown.token_endpoint as string, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "refresh_token",
      client_id: process.env.AUTH_KEYCLOAK_ID!,
      client_secret: process.env.AUTH_KEYCLOAK_SECRET!,
      refresh_token: refreshToken,
    }),
  });
  const tokens = await res.json();
  if (!res.ok) throw tokens;
  return tokens as {
    access_token: string;
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
  providers: [
    Keycloak({
      clientId: process.env.AUTH_KEYCLOAK_ID,
      clientSecret: process.env.AUTH_KEYCLOAK_SECRET,
      issuer: ISSUER,
    }),
  ],
  callbacks: {
    async jwt({ token, account }) {
      const t = token as AppToken & Record<string, unknown>;
      // Initial sign-in: persist the Keycloak tokens onto the (server-only) JWT.
      if (account) {
        t.accessToken = account.access_token;
        t.refreshToken = account.refresh_token;
        t.expiresAt = account.expires_at;
        return token;
      }

      // Still valid (60s safety margin): reuse.
      if (t.expiresAt && Date.now() < (t.expiresAt - 60) * 1000) {
        return token;
      }

      // Expired: refresh, or flag the session so the UI can re-authenticate.
      if (!t.refreshToken) return token;
      try {
        const refreshed = await refreshAccessToken(t.refreshToken);
        t.accessToken = refreshed.access_token;
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
