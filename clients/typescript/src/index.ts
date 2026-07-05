// Unified façade for the go-oikumenea TypeScript SDK (D-ClientSDK).
//
// The per-service clients under ./generated are GENERATED from the Conjure contract
// (scripts/gen-ts-client.sh) — never hand-edited. This file is the only hand-written source: it wires
// every generated service onto one shared HTTP bridge (one base URL + one bearer token), the TS analog
// of the Go SDK's client.New(...) façade (client/client.go).
//
// Every oikumenea service is reachable here, INCLUDING the hermenea ingestion/scheduler endpoints —
// oikumenea reverse-proxies them at /hermenea/v1/* (D-Hermenea), so a single client + base URL reaches
// both oikumenea-native and hermenea-proxied endpoints.

import {
  ConjureError,
  ConjureErrorType,
  DefaultHttpApiBridge,
  type IHttpApiBridge,
  type IUserAgent,
  isConjureError,
} from "conjure-client";

// Re-export the conjure error surface so consumers handle SDK errors without importing
// conjure-client directly. Every call — typed method or generic request() — throws a ConjureError
// (`.status` + `.body`, the SerializableError envelope) on a non-2xx response.
export { ConjureError, ConjureErrorType, isConjureError };

import {
  audit,
  authorization,
  company,
  dataimport,
  document,
  education,
  educationref,
  externalorg,
  finance,
  geo,
  hermenea,
  identityfederation,
  language,
  localization,
  location,
  membership,
  order,
  person,
  platform,
  rank,
  religion,
  tenant,
  vehicle,
} from "./generated";

export * from "./generated";

/** A value or a (possibly async) supplier of it — matches conjure-client's token/baseUrl inputs. */
export type Supplier<T> = () => T;
export type FetchFunction = (
  url: string | Request,
  init?: RequestInit,
) => Promise<Response>;

export interface OikumeneaClientOptions {
  /** Scheme://host[:port] of oikumenea, OR a relative base (e.g. "/api/oikumenea" behind a BFF proxy). */
  baseUrl: string | Supplier<string>;
  /**
   * Bearer token (OIDC/JWT). Omit in the browser when calling through a BFF that injects the token
   * server-side (the web console's pattern). May be a string or a (sync/async) supplier so callers
   * can refresh it per request.
   */
  token?: string | Supplier<string> | Supplier<Promise<string>>;
  /** Override the fetch implementation (e.g. a Node fetch, or one that routes through a proxy). */
  fetch?: FetchFunction;
  /** Identifies the caller in the server's request logs. Defaults to this SDK's name/version. */
  userAgent?: IUserAgent;
}

/** Options for the generic {@link OikumeneaClient.request} escape hatch. */
export interface RequestOptions {
  /** JSON request body; sets `Content-Type: application/json` when present. */
  body?: unknown;
  /** Query string ("a=b" / "?a=b") or a param map (null/undefined values are dropped). */
  query?: string | Record<string, string | number | boolean | null | undefined>;
  /** Extra request headers. */
  headers?: Record<string, string>;
}

/** Every generated service, bound to one shared HTTP bridge. Returned by {@link createOikumeneaClient}. */
export interface OikumeneaClient {
  readonly audit: audit.AuditService;
  readonly authorization: authorization.AuthorizationService;
  readonly company: company.CompanyService;
  readonly document: document.DocumentService;
  readonly education: education.EducationService;
  readonly educationReference: educationref.EducationReferenceService;
  /** External-organizations registry: parties/government/military/NGOs/registrants (D-ExternalOrgs). */
  readonly externalOrg: externalorg.ExternalOrganizationService;
  readonly geo: geo.GeoService;
  readonly identityFederation: identityfederation.IdentityFederationService;
  /** Generic reference-data import endpoint (POST /import/{objectType}); hermenea's loader calls this. */
  readonly import: dataimport.ImportService;
  readonly language: language.LanguageService;
  readonly localization: localization.LocalizationService;
  readonly location: location.LocationService;
  readonly membership: membership.MembershipService;
  readonly order: order.OrderService;
  readonly person: person.PersonService;
  readonly platform: platform.PlatformOpsService;
  readonly platformCatalog: platform.PlatformCatalogService;
  readonly rank: rank.RankService;
  readonly religion: religion.ReligionService;
  readonly tenant: tenant.TenantService;
  readonly vehicle: vehicle.VehicleService;
  /** Bank accounts & payment cards — encrypted IBAN/PAN directory data (D-Finance, M44). */
  readonly finance: finance.FinanceService;
  /** The hermenea ingestion/scheduler control + read API, proxied through oikumenea (D-Hermenea). */
  readonly hermenea: hermenea.HermeneaService;
  /** The underlying conjure HTTP bridge, for advanced use (custom endpoints, binary bodies). */
  readonly bridge: IHttpApiBridge;
  /**
   * Generic request escape hatch for paths without a typed method (e.g. a data-driven UI that
   * iterates over object types). Shares this client's base URL, bearer token and fetch — so it is
   * the SAME transport as the typed methods. Returns parsed JSON (or `undefined` on 204) and throws
   * a {@link ConjureError} on a non-2xx response, exactly like the typed methods.
   */
  request<T = unknown>(method: string, path: string, opts?: RequestOptions): Promise<T>;
}

const DEFAULT_USER_AGENT: IUserAgent = {
  productName: "oikumenea-client",
  productVersion: "0.0.0",
};

/**
 * Build a unified, typed oikumenea client. One transport config (base URL + bearer) powers every
 * service:
 *
 *   const client = createOikumeneaClient({ baseUrl: "https://localhost:8443", token });
 *   const who = await client.identityFederation.whoami();
 *   const runs = await client.hermenea.listRuns();   // reaches hermenea through oikumenea
 */
export function createOikumeneaClient(
  options: OikumeneaClientOptions,
): OikumeneaClient {
  const bridge: IHttpApiBridge = new DefaultHttpApiBridge({
    baseUrl: options.baseUrl,
    userAgent: options.userAgent ?? DEFAULT_USER_AGENT,
    token: options.token,
    fetch: options.fetch as never,
  });

  const resolve = async <V>(v: V | (() => V | Promise<V>)): Promise<V> =>
    typeof v === "function" ? await (v as () => V | Promise<V>)() : v;

  const request = async <T>(
    method: string,
    path: string,
    opts?: RequestOptions,
  ): Promise<T> => {
    const base = await resolve(options.baseUrl);
    let qs = "";
    if (opts?.query != null) {
      if (typeof opts.query === "string") {
        qs = opts.query === "" || opts.query.startsWith("?") ? opts.query : `?${opts.query}`;
      } else {
        const u = new URLSearchParams();
        for (const [k, v] of Object.entries(opts.query)) if (v != null) u.set(k, String(v));
        const s = u.toString();
        qs = s ? `?${s}` : "";
      }
    }
    const headers: Record<string, string> = { Accept: "application/json", ...(opts?.headers ?? {}) };
    const tok = options.token != null ? await resolve(options.token) : undefined;
    if (tok) headers["Authorization"] = `Bearer ${tok}`;
    const hasBody = opts?.body !== undefined;
    if (hasBody && !headers["Content-Type"]) headers["Content-Type"] = "application/json";

    const doFetch = (options.fetch ?? fetch) as typeof fetch;
    let res: Response;
    try {
      res = await doFetch(`${base}${path}${qs}`, {
        method,
        headers,
        body: hasBody ? JSON.stringify(opts!.body) : undefined,
      });
    } catch (e) {
      throw new ConjureError(ConjureErrorType.Network, e);
    }
    if (res.status === 204 || res.status === 205) return undefined as T;
    const text = await res.text();
    let parsed: unknown = text;
    if (text) {
      try {
        parsed = JSON.parse(text);
      } catch {
        /* non-JSON body: keep raw text */
      }
    } else {
      parsed = undefined;
    }
    if (!res.ok) throw new ConjureError(ConjureErrorType.Status, undefined, res.status, parsed as never);
    return parsed as T;
  };

  return {
    audit: new audit.AuditService(bridge),
    authorization: new authorization.AuthorizationService(bridge),
    company: new company.CompanyService(bridge),
    document: new document.DocumentService(bridge),
    education: new education.EducationService(bridge),
    educationReference: new educationref.EducationReferenceService(bridge),
    externalOrg: new externalorg.ExternalOrganizationService(bridge),
    geo: new geo.GeoService(bridge),
    identityFederation: new identityfederation.IdentityFederationService(bridge),
    import: new dataimport.ImportService(bridge),
    language: new language.LanguageService(bridge),
    localization: new localization.LocalizationService(bridge),
    location: new location.LocationService(bridge),
    membership: new membership.MembershipService(bridge),
    order: new order.OrderService(bridge),
    person: new person.PersonService(bridge),
    platform: new platform.PlatformOpsService(bridge),
    platformCatalog: new platform.PlatformCatalogService(bridge),
    rank: new rank.RankService(bridge),
    religion: new religion.ReligionService(bridge),
    tenant: new tenant.TenantService(bridge),
    vehicle: new vehicle.VehicleService(bridge),
    finance: new finance.FinanceService(bridge),
    hermenea: new hermenea.HermeneaService(bridge),
    bridge,
    request,
  };
}
