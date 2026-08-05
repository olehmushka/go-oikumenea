import type { Provider } from "next-auth/providers";
import Keycloak from "next-auth/providers/keycloak";
import Google from "next-auth/providers/google";
import GitLab from "next-auth/providers/gitlab";
import Okta from "next-auth/providers/okta";
import MicrosoftEntraID from "next-auth/providers/microsoft-entra-id";

/**
 * The console's IdP registry (D-MultiIdPExamples).
 *
 * A provider is OFFERED IFF its credentials are present in the environment, so enabling one is a
 * `.env` change and never a code edit — the console-side mirror of the backend's `idp.issuers[]`.
 *
 * Two things every entry must get right:
 *
 * 1. `forward` — WHICH token the BFF sends to go-oikumenea. The service validates a JWT against
 *    `idp.issuers[]`, so the forwarded token must BE a JWT whose `iss` and `aud` the service pins.
 *    Keycloak's ACCESS token qualifies (the realm's audience mapper stamps `aud: oikumenea`), but a
 *    public IdP's access token is generally an opaque, unverifiable string — for those the ID TOKEN
 *    is the JWT, and its `aud` is this console's OAuth client id. Getting this wrong yields a console
 *    that logs in happily and then 401s on every API call.
 * 2. `issuer` — must equal the token's `iss` EXACTLY and match an `idp.issuers[].issuer` entry on the
 *    backend, since that string is both the routing key and half of the `(issuer, subject)` identity.
 *
 * GitHub is deliberately absent. It is an OAuth2 provider, not an OIDC one: it issues no ID token and
 * publishes no JWKS, so there is nothing here the service could verify. GitHub reaches oikumenea by
 * being brokered through Keycloak (Example A in `deploy/oauth/README.md`), where Keycloak remains the
 * issuer. Adding it here would render a button that produces a permanently-401 session.
 */

/** Which of the OAuth response's tokens the BFF forwards as the API bearer. */
export type ForwardedToken = "access_token" | "id_token";

export interface ConsoleProvider {
  /** Auth.js provider id — also the `/api/auth/callback/<id>` path segment registered with the IdP. */
  id: string;
  /** Button label on /login. */
  label: string;
  forward: ForwardedToken;
  /** The token's `iss`; used for refresh discovery and to mirror the backend's issuer entry. */
  issuer: string;
  /**
   * Env-var prefix holding this provider's credentials (`<prefix>_ID` / `<prefix>_SECRET`), needed by
   * the refresh_token grant. Carried explicitly rather than derived from `id`, because a generic OIDC
   * slot's variables are `AUTH_OIDC_<NAME>_*` while its provider id is just `<name>`.
   */
  envPrefix: string;
  factory: () => Provider;
}

const env = (k: string) => process.env[k]?.trim() || undefined;

/**
 * Every provider the console knows how to speak to. Each entry returns undefined when its
 * credentials are absent, which is what makes the offered set env-driven.
 */
function candidates(): (ConsoleProvider | undefined)[] {
  return [
    // --- Keycloak: the default, and the BROKER in Example A (Google/GitHub/... federate INTO it, so
    // the issuer stays Keycloak and no extra entry is needed here for the brokered providers).
    (() => {
      const [id, secret, issuer] = [
        env("AUTH_KEYCLOAK_ID"),
        env("AUTH_KEYCLOAK_SECRET"),
        env("AUTH_KEYCLOAK_ISSUER"),
      ];
      if (!id || !secret || !issuer) return undefined;
      return {
        id: "keycloak",
        label: "Keycloak",
        envPrefix: "AUTH_KEYCLOAK",
        // Keycloak's realm audience mapper puts `aud: oikumenea` on the ACCESS token.
        forward: "access_token" as const,
        issuer,
        factory: () => Keycloak({ clientId: id, clientSecret: secret, issuer }),
      };
    })(),

    // --- Google (direct, Example B). Access tokens are opaque `ya29.*` strings, so the ID token is
    // what the service can verify; its `aud` is this client id, which install.yml must pin.
    (() => {
      const [id, secret] = [env("AUTH_GOOGLE_ID"), env("AUTH_GOOGLE_SECRET")];
      if (!id || !secret) return undefined;
      return {
        id: "google",
        label: "Google",
        envPrefix: "AUTH_GOOGLE",
        forward: "id_token" as const,
        issuer: "https://accounts.google.com",
        factory: () =>
          Google({
            clientId: id,
            clientSecret: secret,
            // offline access + consent is the only way Google returns a refresh_token; without it the
            // session simply expires and the user signs in again (handled, just less pleasant).
            authorization: {
              params: { access_type: "offline", prompt: "consent", scope: "openid email profile" },
            },
          }),
      };
    })(),

    // --- Microsoft Entra ID. The issuer is tenant-scoped and MUST include the /v2.0 suffix — the v1
    // issuer (sts.windows.net) mints a differently-shaped token the v2 discovery document won't verify.
    (() => {
      const [id, secret, issuer] = [
        env("AUTH_ENTRA_ID"),
        env("AUTH_ENTRA_SECRET"),
        env("AUTH_ENTRA_ISSUER"),
      ];
      if (!id || !secret || !issuer) return undefined;
      return {
        id: "entra",
        label: "Microsoft Entra ID",
        envPrefix: "AUTH_ENTRA",
        forward: "id_token" as const,
        issuer,
        factory: () =>
          MicrosoftEntraID({
            id: "entra",
            name: "Microsoft Entra ID",
            clientId: id,
            clientSecret: secret,
            issuer,
          }),
      };
    })(),

    // --- GitLab. Self-managed instances change the issuer, so it is configurable rather than pinned.
    (() => {
      const [id, secret] = [env("AUTH_GITLAB_ID"), env("AUTH_GITLAB_SECRET")];
      if (!id || !secret) return undefined;
      const issuer = env("AUTH_GITLAB_ISSUER") ?? "https://gitlab.com";
      return {
        id: "gitlab",
        label: "GitLab",
        envPrefix: "AUTH_GITLAB",
        forward: "id_token" as const,
        issuer,
        factory: () =>
          GitLab({
            clientId: id,
            clientSecret: secret,
            // GitLab returns an id_token only when `openid` is requested explicitly.
            authorization: { params: { scope: "openid email profile" } },
          }),
      };
    })(),

    // --- Okta / Auth0-shaped tenant. The issuer is the tenant (or custom authorization server) URL.
    (() => {
      const [id, secret, issuer] = [
        env("AUTH_OKTA_ID"),
        env("AUTH_OKTA_SECRET"),
        env("AUTH_OKTA_ISSUER"),
      ];
      if (!id || !secret || !issuer) return undefined;
      return {
        id: "okta",
        label: "Okta",
        envPrefix: "AUTH_OKTA",
        forward: "id_token" as const,
        issuer,
        factory: () => Okta({ clientId: id, clientSecret: secret, issuer }),
      };
    })(),
  ];
}

/**
 * Generic OIDC slots: `AUTH_OIDC_<NAME>_ISSUER` + `_ID` + `_SECRET` registers ANY standards-compliant
 * OIDC provider with no edit to this file — the direct-topology escape hatch, so the named entries
 * above are conveniences rather than the supported set. `<NAME>` becomes the Auth.js provider id
 * (lowercased), hence the callback path `/api/auth/callback/<name>`, and `_LABEL` overrides the
 * button text.
 *
 *   AUTH_OIDC_AUTHENTIK_ISSUER="https://id.example.org/application/o/oikumenea/"
 *   AUTH_OIDC_AUTHENTIK_ID="..."   AUTH_OIDC_AUTHENTIK_SECRET="..."   AUTH_OIDC_AUTHENTIK_LABEL="Authentik"
 *
 * Endpoints come from the issuer's discovery document (Auth.js fetches it), so nothing here needs to
 * know the provider. It forwards the ID token, like every other public IdP.
 */
function genericOidcProviders(): ConsoleProvider[] {
  const names = new Set<string>();
  for (const key of Object.keys(process.env)) {
    const m = /^AUTH_OIDC_(.+)_ISSUER$/.exec(key);
    if (m) names.add(m[1]);
  }
  const out: ConsoleProvider[] = [];
  for (const name of names) {
    const [issuer, id, secret] = [
      env(`AUTH_OIDC_${name}_ISSUER`),
      env(`AUTH_OIDC_${name}_ID`),
      env(`AUTH_OIDC_${name}_SECRET`),
    ];
    if (!issuer || !id || !secret) continue;
    const slug = name.toLowerCase();
    out.push({
      id: slug,
      label: env(`AUTH_OIDC_${name}_LABEL`) ?? name,
      forward: "id_token",
      issuer,
      envPrefix: `AUTH_OIDC_${name}`,
      factory: () => ({
        id: slug,
        name: env(`AUTH_OIDC_${name}_LABEL`) ?? name,
        type: "oidc",
        issuer,
        clientId: id,
        clientSecret: secret,
        authorization: { params: { scope: "openid email profile" } },
      }),
    });
  }
  return out;
}

/** The providers this deployment offers, in registry order. */
export const consoleProviders: ConsoleProvider[] = [
  ...candidates().filter((p): p is ConsoleProvider => p !== undefined),
  ...genericOidcProviders(),
];

/** Index by Auth.js provider id, so the jwt callback can look up how to treat a sign-in. */
export const providerById: Record<string, ConsoleProvider> = Object.fromEntries(
  consoleProviders.map((p) => [p.id, p]),
);
