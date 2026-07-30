// The ontology registry — one data-driven description of each Object/Link type that powers the whole
// workspace: the explorer table, the command-palette fan-out search, the universal object view, the
// link/graph traversal, and the ontology browser. It is the human-facing mirror of D-Ontology
// (docs/ontology-mapping.md) and is keyed by the RID entity_type token (see ./rid).
//
// Isomorphic on purpose: imports only pure helpers (pickLabel, rid) so it can be used from server
// components (explorer/object pages) AND the browser bundle (palette/drawer). It carries NO JSX —
// it exposes string/number accessors + render hints, and the components do the rendering. This also
// keeps it serialization-safe: the server calls these accessors and passes plain data to clients.

import { pickLabel, getActiveLocale, type LocaleMap } from "@/lib/i18n";
import { ridTail } from "./rid";

export type Tone = "slate" | "green" | "amber" | "red" | "indigo";

/** A row as returned by a list endpoint (shape varies per type; we read it via accessors). */
export type Row = Record<string, unknown> & { id: string };

export interface ColumnDef {
  key: string;
  header: string;
  value: (row: Row) => string | number | undefined;
  render?: "mono" | "pill" | "text";
  tone?: (row: Row) => Tone;
  align?: "right";
}

export interface PropertyDef {
  label: string;
  value: (obj: Row) => string | number | undefined;
  render?: "mono" | "pill" | "text";
  tone?: (obj: Row) => Tone;
}

/** One row in a Links panel — a related object you can open (/o/<id>) or traverse in the graph. */
export interface LinkRow {
  id: string;
  label: string;
  sub?: string;
  tone?: Tone;
}

export interface LinkDef {
  label: string;
  /** the object type the rows point at (for the type badge + graph node typing) */
  targetType?: string;
  path: (id: string) => string;
  /** sourceId is the object the links are resolved for — needed to pick the counterpart of a
   * symmetric/directional person↔person link. Most parsers ignore it. */
  parse: (res: unknown, sourceId: string) => LinkRow[];
}

/** A generic, registry-driven action (drawer/object view). Rich edits reuse the bespoke *Forms.tsx. */
export interface ActionDef {
  key: string;
  label: string;
  method: "POST" | "PUT" | "DELETE";
  path: (id: string) => string;
  body?: () => unknown;
  confirm?: string;
  danger?: boolean;
  /** hide the action unless this returns true for the object (cosmetic gate, not authorization) */
  appliesTo?: (obj: Row) => boolean;
}

export interface ListDef {
  path: string;
  /** default query string incl. leading "?", e.g. "?pageSize=50" */
  search?: string;
  /**
   * when set, the explore page shows a search box whose text is sent to the backend as this query
   * param for a name/code substring match (e.g. "query" for languoids). Handy for large catalogs to
   * jump straight to a row; pairs with keyset pagination (nextPageToken) for browsing.
   */
  searchParam?: string;
  /**
   * when set, the list endpoint requires an `org` RID (D-TenantOrganizations, M40 — a fully-unscoped
   * listing is rejected). The explore page shows an organization picker and injects `?org=<rid>`;
   * nothing is listed until an org is chosen.
   */
  orgScoped?: boolean;
  parse: (res: unknown) => { rows: Row[]; nextPageToken?: string };
}

/** The kinds mirror pkg/facet's Kind verbatim (D-ObjectFacets); a facet is declared once by the
 *  module that owns the table and consumed twice — as a list filter here and as M57's groupBy key. */
export type FilterKind = "enum" | "ref" | "date-range" | "bool" | "numeric-range";

/** Which picker loads a `ref` filter's options. A token, not an imported EntityKind: importing from
 *  a "use client" module would drag it into a registry that server components import. */
export type RefControl =
  | "org"
  | "unit"
  | "country"
  | "person"
  | "rank"
  | "domain"
  | "unitKind"
  | "orderType"
  | "documentType"
  | "position";

/**
 * One filterable dimension of an object type — the console half of a `pkg/facet` Facet.
 *
 * `params` carries the EXACT contract arg name(s) rather than re-deriving them from `key` + `kind`.
 * The derivation has live exceptions (unit.level pins the pre-existing scalar `level`;
 * membership.effectiveFrom pins effectiveFromAfter/Before), so re-implementing Facet.Args() here
 * would be a second copy of the rule that can itself drift. Writing the resolved names down makes
 * the guard a set-equality — and makes the URL builder param-driven, with no per-kind branching.
 *
 * ARITY COMES FROM `params`, NEVER FROM `kind`: unit.level is a numeric-range with ONE param. A bar
 * that renders a min/max pair off the kind would send args the contract does not ship.
 *
 * FORMAT CONTRACT: keep `key`, `kind` and `params` as literal string/array literals. pkg/facet's
 * console_test.go parses this file (the plaintext_test.go technique) and holds every entry against
 * the catalog in both directions; a computed value blinds the parser, so it fails the parse rather
 * than passing.
 */
export interface FilterDef {
  /** pkg/facet Facet.Key — also M57's groupBy token */
  key: string;
  kind: FilterKind;
  /** English source string, rendered through <T>/tg() (D-i18n) */
  label: string;
  /** the contract query-arg name(s), in Facet.Args() order */
  params: string[];
  /** enum only: the CHECK set in CHART ORDER (never re-sorted alphabetically or by frequency) */
  values?: { value: string; label: string }[];
  /** ref only */
  control?: RefControl;
  /** ref only: the param whose current value scopes this one's options (domain → unitKind) */
  dependsOn?: string;
  /** the arg is NON-optional in the contract (unit.org — listUnits rejects an unscoped listing) */
  required?: boolean;
  /**
   * Facet.ReadPermission — the inherited read code (D-ObjectFacets rule 2). Empty for every facet
   * today (all are pii:none/basic); the bar hides a filter whose code the caller lacks, which is
   * cosmetic only: the server omits the facet regardless.
   */
  requires?: string;
  /**
   * How the SERVER buckets this facet (pkg/facet `Buckets.Strategy`), declared exactly where `kind`
   * does not imply it: a `date-range` is either calendar `dateTrunc` months or declared `bands`
   * (person.birthdate's age bands), and a `numeric-range` is always `bands`. enum / bool / ref imply
   * identity / bool / topN and must NOT declare it.
   *
   * M57's click-through needs the distinction to turn a bucket key back into a filter — `2026-03` is
   * a month, `25-34` is an age band, and the inverse of an age band is a birthdate range. The Go
   * guard holds this against the catalog, so it cannot drift into decoration.
   */
  buckets?: "bands" | "dateTrunc";
  /** the SQL semantics an operator would otherwise reverse-engineer from a surprising count */
  hint?: string;
}

/**
 * One chart on a type's dashboard — the console half of an M57 facet distribution, and the third
 * consumer of a facet declared once in `pkg/facet` (list filter → stats groupBy → chart).
 *
 * `facet` MUST name a facet the catalog declares for this type: the console asks for exactly the
 * facets it draws (the `facets` CSV is the difference between an 11-second and a 3-second dashboard
 * at root reach), and an undeclared key is a **400 on the whole request** — one stale chart would
 * blank the entire dashboard, not itself. `pkg/facet/dashboard_test.go` holds this.
 *
 * FORMAT CONTRACT, as for FilterDef: keep `key`, `form` and `facet` literal strings — the guard
 * parses this file with regexes and a computed value blinds the parse.
 */
export interface ChartDef {
  /** stable id: the React key and the guard's subject */
  key: string;
  /** English source string, rendered through <T> (D-i18n) */
  title: string;
  form: "tiles" | "bar" | "donut" | "histogram" | "pyramid" | "stat";
  /** the pkg/facet Facet.Key this chart draws */
  facet: string;
  /** bar only — vertical for ORDERED short categories (levels, rank seniority), else horizontal */
  orientation?: "horizontal" | "vertical";
  /** per-bucket status colour: the tones Pill paints the same value with in a table */
  tone?: Record<string, Tone>;
  /** donut only: fold past this many slices into `(other)` (the palette has six identity slots) */
  maxSlices?: number;
  /** histogram only: tone the buckets before this month red — they are overdue, not forecast */
  pastDue?: boolean;
  /**
   * pyramid only: the facet distribution is fetched once per value with that value as an extra
   * filter, because a cross-tab is not something a per-facet stats endpoint can answer.
   */
  splitBy?: { param: string; values: string[] };
  /** stat only: a number computed from the distribution (a ratio) or from an extra bounded count */
  derived?: "revocationRate" | "expiringSoon";
  /** the SQL semantics an operator would otherwise reverse-engineer from a surprising count */
  note?: string;
}

/** A type's dashboard: where the aggregate lives, and what to draw from it. */
export interface DashboardDef {
  /** the stats endpoint path — `/stats/<collection>`, never `/<collection>/stats` (httprouter) */
  path: string;
  charts: ChartDef[];
}

export interface ObjectTypeDef {
  type: string;
  kind: "object" | "link" | "action";
  label: string;
  labelPlural: string;
  /** module the type belongs to (for the ontology browser grouping) */
  module: string;
  /**
   * The `<module>.read` permission code that gates VIEWING this type, for cosmetic UI hiding
   * (D-SelfCapabilities). Usually derived from `module` via readCodeFor(); set this only to override
   * the module default (e.g. graph → graph.read, link__has_role → assignment.read). The server still
   * re-decides every request regardless of what the UI drew.
   */
  requires?: string;
  /** one-line description for the ontology browser */
  blurb?: string;
  /** present only for types with an unconditional top-level list (explorable + searchable) */
  list?: ListDef;
  /** single-object fetch for the object view/drawer */
  get?: (id: string) => string;
  title: (obj: Row) => string;
  subtitle?: (obj: Row) => string | undefined;
  columns: ColumnDef[];
  /** the type's facet vocabulary as list filters (D-ObjectFacets / D-ConsoleDashboards, M56) */
  filters?: FilterDef[];
  /** the same vocabulary as charts — `?view=dashboard` on the same URL (M57 ticket 3) */
  dashboard?: DashboardDef;
  properties?: PropertyDef[];
  links?: LinkDef[];
  actions?: ActionDef[];
}

// ── small accessors ─────────────────────────────────────────────────────────
const s = (v: unknown): string | undefined => (v == null ? undefined : String(v));
// Resolves a `locale → text` map against the module-global active UI locale (set by the root layout
// on the server and by LocaleProvider on the client). Switching the UI locale re-renders the views
// that read this, so names re-render in the chosen locale (D-i18n).
const loc = (v: unknown): string => pickLabel(v as LocaleMap, getActiveLocale());
const arr = (res: unknown, key: string): Row[] =>
  (((res as Record<string, unknown>)?.[key] as Row[]) ?? []);
const pageParse = (key: string) => (res: unknown) => ({
  rows: arr(res, key),
  nextPageToken: (res as { nextPageToken?: string })?.nextPageToken,
});
/** catalog endpoints return a bare array */
const listParse = () => (res: unknown) => ({ rows: (res as Row[]) ?? [], nextPageToken: undefined });
/** code-keyed catalogs (no RID id) — surface `code` as the row id so tables/keys work */
const catalogParse = (res: unknown) => ({
  rows: bare(res).map((r) => ({ ...r, id: String(r.id ?? r.code ?? "") })),
  nextPageToken: undefined,
});
/** bare-array list responses (relationship/social endpoints) */
const bare = (res: unknown): Row[] => (Array.isArray(res) ? (res as Row[]) : []);
/** counterpart of a two-ended person link, relative to the person being viewed */
const other = (a: string, b: string, src: string): string => (a === src ? b : a);

const statusTone = (v: unknown): Tone => {
  const up = String(v ?? "").toUpperCase();
  if (["ACTIVE", "ISSUED", "FILLED", "ENABLED"].includes(up)) return "green";
  if (["REVOKED", "PURGED", "ABOLISHED"].includes(up)) return "red";
  if (["SHADOW", "DRAFT", "DEACTIVATED", "SUSPENDED"].includes(up)) return "amber";
  return "slate";
};
/** relationship status tones (active/married green; ended/dissolved slate; else amber) */
const relTone = (v: unknown): Tone => {
  const low = String(v ?? "").toLowerCase();
  if (["active", "married"].includes(low)) return "green";
  if (["ended", "withdrawn", "disestablished", "divorced", "dissolved", "annulled", "widowed"].includes(low)) return "slate";
  return "amber";
};

// ── the registry ────────────────────────────────────────────────────────────
export const OBJECT_TYPES: Record<string, ObjectTypeDef> = {
  person: {
    type: "person",
    kind: "object",
    label: "Person",
    labelPlural: "Persons",
    module: "person",
    blurb: "Instance-global personnel directory; account-optional, holds exactly one rank.",
    list: {
      path: "/person/v1/persons",
      search: "?pageSize=50",
      // R-21 keeps List and Search separate plan shapes; `query` routes to SearchPersons (trigram),
      // the structural filters below to ListPersons. pkg/facet classifies it ClassSearch.
      searchParam: "query",
      parse: pageParse("persons"),
    },
    get: (id) => `/person/v1/persons/${id}`,
    title: (p) => s(p.displayName) || s(p.code) || ridTail(p.id),
    subtitle: (p) => s(p.code),
    columns: [
      { key: "code", header: "Code", value: (p) => s(p.code) || ridTail(p.id), render: "mono" },
      { key: "displayName", header: "Display name", value: (p) => s(p.displayName) },
      { key: "sex", header: "Sex", value: (p) => s(p.sex) },
      { key: "birthdate", header: "Birthdate", value: (p) => s(p.birthdate) },
      { key: "status", header: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
    ],
    filters: [
      {
        key: "sex", kind: "enum", label: "Sex", params: ["sex"],
        values: [
          { value: "not_known", label: "Not known" },
          { value: "male", label: "Male" },
          { value: "female", label: "Female" },
          { value: "not_applicable", label: "Not applicable" },
        ],
      },
      {
        key: "status", kind: "enum", label: "Status", params: ["status"],
        values: [
          { value: "active", label: "Active" },
          { value: "deactivated", label: "Deactivated" },
          { value: "provisional", label: "Provisional" },
          { value: "purged", label: "Purged" },
        ],
      },
      {
        key: "birthdate", kind: "date-range", label: "Birthdate",
        params: ["birthdateFrom", "birthdateTo"],
        buckets: "bands",
        hint: "Setting either bound excludes unknown birthdates.",
      },
      { key: "countryOfBirth", kind: "ref", label: "Country of birth", params: ["countryOfBirth"], control: "country" },
      { key: "rankId", kind: "ref", label: "Rank", params: ["rankId"], control: "rank" },
      {
        key: "unitId", kind: "ref", label: "Unit", params: ["unitId"], control: "unit",
        hint: "Includes the whole subtree.",
      },
      { key: "hasAccount", kind: "bool", label: "Has account", params: ["hasAccount"] },
    ],
    dashboard: {
      path: "/person/v1/stats/persons",
      charts: [
        {
          key: "status", title: "Directory", form: "tiles", facet: "status",
          tone: { active: "green", deactivated: "amber", provisional: "slate", purged: "red" },
        },
        {
          key: "pyramid", title: "Age structure", form: "pyramid", facet: "birthdate",
          splitBy: { param: "sex", values: ["male", "female"] },
          note: "Age at today's date; persons with no birthdate are counted beside the chart, never inside a band.",
        },
        { key: "sex", title: "Sex", form: "donut", facet: "sex" },
        {
          key: "rank", title: "Rank distribution", form: "bar", facet: "rankId", orientation: "vertical",
          note: "The fifteen most-held ranks, ordered by seniority rather than by count.",
        },
        { key: "units", title: "Top units", form: "bar", facet: "unitId", orientation: "horizontal" },
        {
          key: "countryOfBirth", title: "Country of birth", form: "bar", facet: "countryOfBirth",
          orientation: "horizontal",
        },
      ],
    },
    properties: [
      { label: "Given", value: (p) => s(p.given) },
      { label: "Surname", value: (p) => s(p.surname) },
      { label: "Patronymic", value: (p) => s(p.patronymic) },
      { label: "Birthdate", value: (p) => s(p.birthdate) },
      { label: "Sex", value: (p) => s(p.sex) },
      { label: "Country of birth", value: (p) => s(p.countryOfBirth) },
      { label: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
    ],
    links: [
      {
        label: "Memberships",
        targetType: "unit",
        path: (id) => `/membership/v1/persons/${id}/memberships`,
        parse: (r) => arr(r, "memberships").map((m) => ({ id: s(m.unitId)!, label: ridTail(s(m.unitId)!), sub: s(m.status), tone: statusTone(m.status) })),
      },
      {
        label: "Orders",
        targetType: "order",
        path: (id) => `/order/v1/persons/${id}/orders`,
        parse: (r) => arr(r, "orders").map((o) => ({ id: s(o.id)!, label: s(o.number) || ridTail(s(o.id)!), sub: s(o.status), tone: statusTone(o.status) })),
      },
      {
        label: "Documents",
        path: (id) => `/document/v1/persons/${id}/documents`,
        parse: (r) => arr(r, "documents").map((d) => ({ id: s(d.id)!, label: s(d.number) || ridTail(s(d.id)!), sub: s(d.status) })),
      },
      {
        label: "Social accounts",
        targetType: "social_account",
        path: (id) => `/person/v1/persons/${id}/social-accounts`,
        parse: (r) => bare(r).map((a) => ({ id: s(a.id)!, label: `@${s(a.handle)}`, sub: s(a.platformCode), tone: a.platformVerified ? "green" : "slate" })),
      },
      // Person↔person relationships (M14): each row carries both ends; we surface the counterpart so the
      // graph/links point person→person. `sourceId` is the person being viewed.
      {
        label: "Partnerships",
        targetType: "person",
        path: (id) => `/person/v1/persons/${id}/partnerships`,
        parse: (r, src) => bare(r).map((p) => ({ id: other(s(p.personIdA)!, s(p.personIdB)!, src), label: ridTail(other(s(p.personIdA)!, s(p.personIdB)!, src)), sub: s(p.status), tone: relTone(p.status) })),
      },
      {
        label: "Kin",
        targetType: "person",
        path: (id) => `/person/v1/persons/${id}/kinships`,
        parse: (r, src) => bare(r).map((k) => ({ id: other(s(k.parentId)!, s(k.childId)!, src), label: ridTail(other(s(k.parentId)!, s(k.childId)!, src)), sub: s(k.parentId) === src ? "child" : "parent", tone: relTone(k.status) })),
      },
      {
        label: "Guardianships",
        targetType: "person",
        path: (id) => `/person/v1/persons/${id}/guardianships`,
        parse: (r, src) => bare(r).map((g) => ({ id: other(s(g.guardianId)!, s(g.wardId)!, src), label: ridTail(other(s(g.guardianId)!, s(g.wardId)!, src)), sub: s(g.guardianId) === src ? "ward" : "guardian", tone: relTone(g.status) })),
      },
      {
        label: "Sponsorships",
        targetType: "person",
        path: (id) => `/person/v1/persons/${id}/sponsorships`,
        parse: (r, src) => bare(r).map((sp) => ({ id: other(s(sp.sponsorId)!, s(sp.sponsoredId)!, src), label: ridTail(other(s(sp.sponsorId)!, s(sp.sponsoredId)!, src)), sub: s(sp.relationCode), tone: relTone(sp.status) })),
      },
      {
        label: "Next of kin",
        targetType: "person",
        path: (id) => `/person/v1/persons/${id}/next-of-kin`,
        parse: (r, src) => bare(r).map((n) => ({ id: other(s(n.subjectId)!, s(n.contactId)!, src), label: ridTail(other(s(n.subjectId)!, s(n.contactId)!, src)), sub: `#${s(n.priority)}`, tone: relTone(n.status) })),
      },
      {
        label: "Associations",
        targetType: "person",
        path: (id) => `/person/v1/persons/${id}/associations`,
        parse: (r, src) => bare(r).map((a) => ({ id: other(s(a.personIdA)!, s(a.personIdB)!, src), label: ridTail(other(s(a.personIdA)!, s(a.personIdB)!, src)), sub: s(a.kind), tone: s(a.kind) === "no_contact" ? "slate" : relTone(a.status) })),
      },
      // Education (M20/M21): the person's enrollments + the other education bindings, each pointing at
      // its education object (institution / building / research group / grant / publication / body / …).
      {
        label: "Education",
        targetType: "institution",
        path: (id) => `/education/v1/persons/${id}/enrollments`,
        parse: (r) => arr(r, "enrollments").map((e) => ({ id: s(e.institutionId)!, label: ridTail(s(e.institutionId)!), sub: s(e.status), tone: relTone(e.status) })),
      },
      {
        label: "Dormitory stays",
        targetType: "building",
        path: (id) => `/education/v1/persons/${id}/dormitory-stays`,
        parse: (r) => arr(r, "dormitoryStays").map((d) => ({ id: s(d.buildingId)!, label: ridTail(s(d.buildingId)!), sub: s(d.status), tone: relTone(d.status) })),
      },
      {
        label: "Appointments",
        targetType: "education_position",
        path: (id) => `/education/v1/persons/${id}/appointments`,
        parse: (r) => arr(r, "appointments").map((a) => ({ id: s(a.positionId)!, label: s(a.positionTitle) || ridTail(s(a.positionId)!), sub: s(a.status), tone: relTone(a.status) })),
      },
      {
        label: "Research memberships",
        targetType: "research_group",
        path: (id) => `/education/v1/persons/${id}/research-memberships`,
        parse: (r) => arr(r, "memberships").map((m) => ({ id: s(m.groupId)!, label: ridTail(s(m.groupId)!), sub: s(m.role) || s(m.status), tone: relTone(m.status) })),
      },
      {
        label: "Grant holdings",
        targetType: "grant",
        path: (id) => `/education/v1/persons/${id}/grant-holdings`,
        parse: (r) => arr(r, "holdings").map((h) => ({ id: s(h.grantId)!, label: ridTail(s(h.grantId)!), sub: s(h.role) || s(h.status), tone: relTone(h.status) })),
      },
      {
        label: "Governance memberships",
        targetType: "governance_body",
        path: (id) => `/education/v1/persons/${id}/governance-memberships`,
        parse: (r) => arr(r, "memberships").map((m) => ({ id: s(m.bodyId)!, label: ridTail(s(m.bodyId)!), sub: s(m.roleInBody) || s(m.status), tone: relTone(m.status) })),
      },
      {
        label: "Publication authorships",
        targetType: "publication",
        path: (id) => `/education/v1/persons/${id}/publication-authorships`,
        parse: (r) => arr(r, "authorships").map((a) => ({ id: s(a.publicationId)!, label: ridTail(s(a.publicationId)!), sub: a.corresponding ? "corresponding" : undefined })),
      },
      {
        label: "Qualification awards",
        targetType: "qualification",
        path: (id) => `/education/v1/persons/${id}/qualification-awards`,
        parse: (r) => arr(r, "awards").map((a) => ({ id: s(a.qualificationId)!, label: ridTail(s(a.qualificationId)!), sub: s(a.status), tone: relTone(a.status) })),
      },
      {
        label: "Scholarship awards",
        targetType: "scholarship",
        path: (id) => `/education/v1/persons/${id}/scholarship-awards`,
        parse: (r) => arr(r, "awards").map((a) => ({ id: s(a.scholarshipId)!, label: ridTail(s(a.scholarshipId)!), sub: s(a.status), tone: relTone(a.status) })),
      },
    ],
    actions: [
      { key: "deactivate", label: "Deactivate", method: "POST", path: (id) => `/person/v1/persons/${id}/deactivate`, confirm: "Deactivate this person?", appliesTo: (p) => String(p.status ?? "").toUpperCase() === "ACTIVE" },
      { key: "reactivate", label: "Reactivate", method: "POST", path: (id) => `/person/v1/persons/${id}/reactivate`, appliesTo: (p) => String(p.status ?? "").toUpperCase() !== "ACTIVE" },
    ],
  },

  unit: {
    type: "unit",
    kind: "object",
    label: "Unit",
    labelPlural: "Units",
    module: "tenant",
    blurb: "Units as a DAG (multi-parent, multi-root); public/shadow visibility. Feeds the PDP.",
    list: { path: "/tenant/v1/units", search: "?pageSize=50", orgScoped: true, parse: pageParse("units") },
    get: (id) => `/tenant/v1/units/${id}`,
    title: (u) => s(u.code) || ridTail(u.id),
    subtitle: (u) => loc(u.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (u) => s(u.code), render: "mono" },
      { key: "name", header: "Name", value: (u) => loc(u.name) },
      { key: "kindId", header: "Kind", value: (u) => s(u.kindId), render: "mono" },
      { key: "level", header: "Level", value: (u) => s(u.level) },
      { key: "visibility", header: "Visibility", value: (u) => s(u.visibility), render: "pill", tone: (u) => statusTone(u.visibility) },
      { key: "state", header: "State", value: (u) => s(u.state), render: "pill", tone: (u) => statusTone(u.state) },
    ],
    filters: [
      // org is REQUIRED — listUnits rejects a fully-unscoped listing (D-TenantOrganizations, M40).
      { key: "org", kind: "ref", label: "Organization", params: ["org"], control: "org", required: true },
      { key: "domain", kind: "ref", label: "Domain", params: ["domain"], control: "domain" },
      {
        key: "unitKind", kind: "ref", label: "Unit kind", params: ["unitKind"], control: "unitKind",
        // unit_kinds are domain-scoped, and the catalog endpoint requires the domain arg.
        dependsOn: "domain",
      },
      {
        // numeric-range with ONE param: the contract's `level` predates the vocabulary and is an
        // exact match, so the facet pins it via ArgOverride. M57 bands the same column for charts.
        key: "level", kind: "numeric-range", label: "Level", params: ["level"],
        buckets: "bands",
        hint: "Exact depth in the unit hierarchy.",
      },
      {
        key: "visibility", kind: "enum", label: "Visibility", params: ["visibility"],
        values: [
          { value: "public", label: "Public" },
          { value: "shadow", label: "Shadow" },
        ],
      },
      {
        key: "state", kind: "enum", label: "State", params: ["state"],
        values: [
          { value: "active", label: "Active" },
          { value: "suspended", label: "Suspended" },
          { value: "archived", label: "Archived" },
        ],
      },
      { key: "pdpScoped", kind: "bool", label: "PDP-scoped", params: ["pdpScoped"] },
    ],
    dashboard: {
      path: "/tenant/v1/stats/units",
      charts: [
        {
          key: "state", title: "Units", form: "tiles", facet: "state",
          tone: { active: "green", suspended: "amber", archived: "slate" },
        },
        {
          key: "level", title: "Units per level", form: "bar", facet: "level", orientation: "vertical",
          note: "Depth bands, shallowest first — the org chart's width profile.",
        },
        { key: "kind", title: "Kind mix", form: "donut", facet: "unitKind" },
        {
          // A bar rather than a donut: the shadow count is a governance number an operator reads
          // exactly, so it carries its own count beside the mark (facets.md ③).
          key: "visibility", title: "Public / shadow", form: "bar", facet: "visibility",
          orientation: "horizontal", tone: { public: "slate", shadow: "amber" },
          note: "A shadow unit is listed only for a subject whose read reaches it (L-Visibility).",
        },
      ],
    },
    properties: [
      { label: "Name", value: (u) => loc(u.name) },
      { label: "Code", value: (u) => s(u.code), render: "mono" },
      { label: "Kind", value: (u) => s(u.kindId), render: "mono" },
      { label: "Level", value: (u) => s(u.level) },
      { label: "Visibility", value: (u) => s(u.visibility), render: "pill", tone: (u) => statusTone(u.visibility) },
      { label: "State", value: (u) => s(u.state), render: "pill", tone: (u) => statusTone(u.state) },
    ],
    links: [
      { label: "Parents (ancestors)", targetType: "unit", path: (id) => `/tenant/v1/units/${id}/ancestors`, parse: (r) => arr(r, "units").map((u) => ({ id: s(u.id)!, label: s(u.code) || ridTail(s(u.id)!), sub: loc(u.name) })) },
      { label: "Children (descendants)", targetType: "unit", path: (id) => `/tenant/v1/units/${id}/descendants`, parse: (r) => arr(r, "units").map((u) => ({ id: s(u.id)!, label: s(u.code) || ridTail(s(u.id)!), sub: loc(u.name) })) },
      { label: "Positions", targetType: "position", path: (id) => `/membership/v1/units/${id}/positions`, parse: (r) => arr(r, "positions").map((p) => ({ id: s(p.id)!, label: s(p.code) || ridTail(s(p.id)!), sub: s(p.status), tone: statusTone(p.status) })) },
      { label: "Members", targetType: "person", path: (id) => `/membership/v1/units/${id}/members`, parse: (r) => arr(r, "memberships").map((m) => ({ id: s(m.personId)!, label: ridTail(s(m.personId)!), sub: s(m.status), tone: statusTone(m.status) })) },
      { label: "Orders", targetType: "order", path: (id) => `/order/v1/units/${id}/orders`, parse: (r) => arr(r, "orders").map((o) => ({ id: s(o.id)!, label: s(o.number) || ridTail(s(o.id)!), sub: s(o.status), tone: statusTone(o.status) })) },
    ],
  },

  order: {
    type: "order",
    kind: "object",
    label: "Order",
    labelPlural: "Orders",
    module: "order",
    blurb: "Administrative orders (наказ): the legal basis for status changes; effects on issue.",
    list: { path: "/order/v1/orders", search: "?pageSize=50", parse: pageParse("orders") },
    get: (id) => `/order/v1/orders/${id}`,
    title: (o) => (s(o.number) ? `Order ${s(o.number)}` : `Order ${ridTail(o.id)}`),
    subtitle: (o) => s(o.status),
    columns: [
      { key: "number", header: "Number", value: (o) => s(o.number) || ridTail(o.id), render: "mono" },
      { key: "issuingUnitId", header: "Issuing unit", value: (o) => (s(o.issuingUnitId) ? ridTail(s(o.issuingUnitId)!) : undefined), render: "mono" },
      { key: "issuedOn", header: "Issued on", value: (o) => s(o.issuedOn) },
      { key: "items", header: "Items", value: (o) => (o.items as unknown[])?.length ?? 0 },
      { key: "status", header: "Status", value: (o) => s(o.status), render: "pill", tone: (o) => statusTone(o.status) },
    ],
    filters: [
      { key: "issuingUnitId", kind: "ref", label: "Issuing unit", params: ["issuingUnitId"], control: "unit" },
      { key: "orderTypeId", kind: "ref", label: "Order type", params: ["orderTypeId"], control: "orderType" },
      {
        key: "status", kind: "enum", label: "Status", params: ["status"],
        values: [
          { value: "draft", label: "Draft" },
          { value: "issued", label: "Issued" },
          { value: "revoked", label: "Revoked" },
        ],
      },
      {
        key: "issuedOn", kind: "date-range", label: "Issued on",
        params: ["issuedOnFrom", "issuedOnTo"],
        buckets: "dateTrunc",
        hint: "Setting either bound excludes drafts (no issue date).",
      },
    ],
    dashboard: {
      path: "/order/v1/stats/orders",
      charts: [
        {
          key: "status", title: "Register", form: "tiles", facet: "status",
          tone: { draft: "amber", issued: "green", revoked: "red" },
        },
        {
          key: "revocationRate", title: "Revocation rate", form: "stat", facet: "status",
          derived: "revocationRate",
          note: "Revoked orders as a share of everything ever issued — the audit-facing number.",
        },
        {
          key: "issuedOn", title: "Orders per month", form: "histogram", facet: "issuedOn",
          note: "An order with no issue date — typically a draft — is counted beside the axis, not on it.",
        },
        {
          key: "types", title: "Type mix", form: "bar", facet: "orderTypeId", orientation: "horizontal",
          note: "An order counts once per type it carries an item of — effects live on the items.",
        },
      ],
    },
    properties: [
      { label: "Number", value: (o) => s(o.number), render: "mono" },
      { label: "Issued on", value: (o) => s(o.issuedOn) },
      { label: "Issuing unit", value: (o) => (o.issuingUnitId ? ridTail(s(o.issuingUnitId)!) : undefined), render: "mono" },
      { label: "Status", value: (o) => s(o.status), render: "pill", tone: (o) => statusTone(o.status) },
    ],
    actions: [
      { key: "issue", label: "Issue", method: "POST", path: (id) => `/order/v1/orders/${id}/issue`, confirm: "Issue this order? Effects apply immediately.", appliesTo: (o) => String(o.status ?? "").toUpperCase() === "DRAFT" },
      { key: "revoke", label: "Revoke", method: "POST", path: (id) => `/order/v1/orders/${id}/revoke`, confirm: "Revoke this order?", danger: true, appliesTo: (o) => String(o.status ?? "").toUpperCase() === "ISSUED" },
    ],
  },

  role: {
    type: "role",
    kind: "object",
    label: "Role",
    labelPlural: "Roles",
    module: "authorization",
    blurb: "RBAC roles; permissions are code, not rows. Assignments scope a role to a unit.",
    list: { path: "/authorization/v1/roles", search: "?pageSize=50", parse: pageParse("roles") },
    get: (id) => `/authorization/v1/roles/${id}`,
    title: (r) => s(r.code) || ridTail(r.id),
    subtitle: (r) => loc(r.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (r) => s(r.code), render: "mono" },
      { key: "name", header: "Name", value: (r) => loc(r.name) },
      { key: "permissions", header: "Permissions", value: (r) => (r.permissions as unknown[])?.length ?? 0 },
      { key: "isBase", header: "Base", value: (r) => (r.isBase ? "base" : ""), render: "pill", tone: () => "indigo" },
    ],
    properties: [
      { label: "Code", value: (r) => s(r.code), render: "mono" },
      { label: "Name", value: (r) => loc(r.name) },
      { label: "Description", value: (r) => loc(r.description) },
      { label: "Permissions", value: (r) => (r.permissions as string[])?.join(", ") },
      { label: "Base role", value: (r) => (r.isBase ? "yes" : "no") },
    ],
  },

  link__has_role: {
    type: "link__has_role",
    kind: "link",
    label: "Assignment",
    labelPlural: "Assignments",
    module: "authorization",
    requires: "assignment.read", // not role.read — assignments are gated separately from role definitions
    blurb: "Reified (person, role, target_unit, scope) link — the PDP's grant. scope∈{unit|subtree}.",
    // NOTE: listAssignments requires exactly one of subjectPersonId/targetUnitId (scoped, like orders &
    // memberships) — so no unconditional global list. Browse them scoped on the Roles & access page,
    // or from a person/unit object view.
    title: (a) => `${ridTail(s(a.subjectPersonId)!)} → ${ridTail(s(a.targetUnitId)!)}`,
    subtitle: (a) => s(a.scope),
    columns: [
      { key: "subject", header: "Subject", value: (a) => ridTail(s(a.subjectPersonId)!), render: "mono" },
      { key: "role", header: "Role", value: (a) => ridTail(s(a.roleId)!), render: "mono" },
      { key: "target", header: "Target unit", value: (a) => ridTail(s(a.targetUnitId)!), render: "mono" },
      { key: "scope", header: "Scope", value: (a) => s(a.scope), render: "pill", tone: (a) => (a.scope === "subtree" ? "indigo" : "slate") },
      { key: "status", header: "Status", value: (a) => (a.revokedAt ? "revoked" : "active"), render: "pill", tone: (a) => (a.revokedAt ? "red" : "green") },
    ],
    properties: [
      { label: "Subject person", value: (a) => s(a.subjectPersonId), render: "mono" },
      { label: "Role", value: (a) => s(a.roleId), render: "mono" },
      { label: "Target unit", value: (a) => s(a.targetUnitId), render: "mono" },
      { label: "Scope", value: (a) => s(a.scope), render: "pill", tone: (a) => (a.scope === "subtree" ? "indigo" : "slate") },
      { label: "Granted at", value: (a) => s(a.grantedAt) },
      { label: "Expires at", value: (a) => s(a.expiresAt) },
      { label: "Revoked at", value: (a) => s(a.revokedAt) },
    ],
  },

  link__member_of: {
    type: "link__member_of",
    kind: "link",
    label: "Membership",
    labelPlural: "Memberships",
    module: "membership",
    blurb: "Reified person ↔ unit belonging, effective-dated, optionally filling a position.",
    // The FIRST faceted reified link (M56 ticket 3). Unlike the per-unit roster and the per-person
    // listing, this top-level list carries NO implicit status default — an ended membership is
    // reachable only here, and a hidden active-only filter would make M57's totalCount disagree
    // with its own status distribution.
    list: { path: "/membership/v1/memberships", search: "?pageSize=50", parse: pageParse("memberships") },
    // No `get`: the contract ships no GET /memberships/{id}. The table suppresses the row-click
    // drawer for a type with no detail endpoint rather than opening one that immediately errors.
    title: (m) => `${ridTail(s(m.personId)!)} → ${ridTail(s(m.unitId)!)}`,
    subtitle: (m) => s(m.status),
    columns: [
      { key: "personId", header: "Person", value: (m) => ridTail(s(m.personId)!), render: "mono" },
      { key: "unitId", header: "Unit", value: (m) => ridTail(s(m.unitId)!), render: "mono" },
      { key: "positionId", header: "Position", value: (m) => (s(m.positionId) ? ridTail(s(m.positionId)!) : undefined), render: "mono" },
      { key: "status", header: "Status", value: (m) => s(m.status), render: "pill", tone: (m) => statusTone(m.status) },
      { key: "effectiveFrom", header: "Effective from", value: (m) => s(m.effectiveFrom) },
      { key: "effectiveTo", header: "Effective to", value: (m) => s(m.effectiveTo) },
    ],
    filters: [
      {
        key: "unitId", kind: "ref", label: "Unit", params: ["unitId"], control: "unit",
        // person.unitId expands to the subtree; this one does not — same control, different SQL.
        hint: "Exact unit — not the subtree.",
      },
      { key: "personId", kind: "ref", label: "Person", params: ["personId"], control: "person" },
      { key: "positionId", kind: "ref", label: "Position", params: ["positionId"], control: "position", dependsOn: "unitId" },
      {
        key: "status", kind: "enum", label: "Status", params: ["status"],
        values: [
          { value: "active", label: "Active" },
          { value: "ended", label: "Ended" },
        ],
      },
      {
        // ArgOverride in the catalog: the contract's after/before predate the From/To convention.
        key: "effectiveFrom", kind: "date-range", label: "Effective from",
        params: ["effectiveFromAfter", "effectiveFromBefore"],
        buckets: "dateTrunc",
      },
    ],
    dashboard: {
      // Deliberately three charts, not five: `personId` is a FILTER facet with no chart behind it,
      // and its top-N over a million distinct persons costs 8.6 s on its own — asking for what is
      // drawn is what keeps this dashboard at ~1.3 s (review-2026-07 § M57 ticket 2).
      path: "/membership/v1/stats/memberships",
      charts: [
        {
          key: "status", title: "Memberships", form: "tiles", facet: "status",
          tone: { active: "green", ended: "slate" },
          note: "The global list applies no implicit status filter, so ended rows are counted here.",
        },
        {
          key: "intake", title: "Joins per month", form: "histogram", facet: "effectiveFrom",
          note: "By the month the membership took effect.",
        },
        {
          key: "billets", title: "Billets held", form: "bar", facet: "positionId",
          orientation: "horizontal",
          note: "The unlabelled bucket is memberships with no billet — a membership without a position is legal.",
        },
      ],
    },
    properties: [
      { label: "Person", value: (m) => s(m.personId), render: "mono" },
      { label: "Unit", value: (m) => s(m.unitId), render: "mono" },
      { label: "Position", value: (m) => s(m.positionId), render: "mono" },
      { label: "Status", value: (m) => s(m.status), render: "pill", tone: (m) => statusTone(m.status) },
      { label: "Effective from", value: (m) => s(m.effectiveFrom) },
      { label: "Effective to", value: (m) => s(m.effectiveTo) },
      { label: "Order item", value: (m) => s(m.orderItemId), render: "mono" },
    ],
  },

  graph: {
    type: "graph",
    kind: "object",
    label: "Graph",
    labelPlural: "Graphs",
    module: "tenant",
    requires: "graph.read", // not unit.read — graph definitions have their own read code
    blurb: "Named hierarchy over units; is_authority_bearing gates the PDP cascade.",
    list: { path: "/tenant/v1/graphs", parse: pageParse("graphs") },
    title: (g) => s(g.code) || ridTail(g.id),
    subtitle: (g) => loc(g.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (g) => s(g.code), render: "mono" },
      { key: "name", header: "Name", value: (g) => loc(g.name) },
      { key: "directoryOnly", header: "Directory-only", value: (g) => (g.isDirectoryOnly ? "yes" : "") },
    ],
    properties: [
      { label: "Code", value: (g) => s(g.code), render: "mono" },
      { label: "Name", value: (g) => loc(g.name) },
      { label: "Directory-only", value: (g) => (g.isDirectoryOnly ? "yes" : "no") },
    ],
  },

  order_type: {
    type: "order_type",
    kind: "object",
    label: "Order type",
    labelPlural: "Order types",
    module: "order",
    blurb: "Instance-admin catalog of order kinds; effect declares the downstream consequence.",
    list: { path: "/order/v1/order-types", parse: listParse() },
    title: (t) => s(t.code) || ridTail(t.id),
    subtitle: (t) => loc(t.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (t) => s(t.code), render: "mono" },
      { key: "name", header: "Name", value: (t) => loc(t.name) },
      { key: "status", header: "Status", value: (t) => s(t.status), render: "pill", tone: (t) => statusTone(t.status) },
    ],
  },

  document_type: {
    type: "document_type",
    kind: "object",
    label: "Document type",
    labelPlural: "Document types",
    module: "document",
    blurb: "Instance-admin catalog for identity papers; metadata only.",
    list: { path: "/document/v1/document-types", parse: listParse() },
    title: (t) => s(t.code) || ridTail(t.id),
    subtitle: (t) => loc(t.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (t) => s(t.code), render: "mono" },
      { key: "name", header: "Name", value: (t) => loc(t.name) },
      { key: "status", header: "Status", value: (t) => s(t.status), render: "pill", tone: (t) => statusTone(t.status) },
    ],
  },

  personal_code_scheme: {
    type: "personal_code_scheme",
    kind: "object",
    label: "Personal-code scheme",
    labelPlural: "Personal-code schemes",
    module: "document",
    blurb: "Instance-admin catalog for personal codes (tax/social-insurance); value is encrypted.",
    list: { path: "/document/v1/personal-code-schemes", parse: listParse() },
    title: (t) => s(t.code) || ridTail(s(t.id) || ""),
    subtitle: (t) => loc(t.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (t) => s(t.code), render: "mono" },
      { key: "name", header: "Name", value: (t) => loc(t.name) },
      { key: "country", header: "Country", value: (t) => s(t.country) },
      { key: "status", header: "Status", value: (t) => s(t.status), render: "pill", tone: (t) => statusTone(t.status) },
    ],
  },

  locale: {
    type: "locale",
    kind: "object",
    label: "Locale",
    labelPlural: "Locales",
    module: "localization",
    blurb: "Instance-admin-managed supported locales (ISO 639-3); the translation store's keys.",
    list: { path: "/localization/v1/locales", parse: pageParse("locales") },
    title: (l) => s(l.code) || "",
    subtitle: (l) => s(l.name),
    columns: [
      { key: "code", header: "Code", value: (l) => s(l.code), render: "mono" },
      { key: "name", header: "Name", value: (l) => s(l.name) },
      { key: "enabled", header: "Enabled", value: (l) => (l.enabled === false ? "" : "yes"), render: "pill", tone: (l) => (l.enabled === false ? "slate" : "green") },
      { key: "default", header: "Default", value: (l) => (l.isDefault ? "default" : "") },
    ],
  },

  // Scoped/child object types — no global list, but get/properties/links so object views & graph work.
  position: {
    type: "position",
    kind: "object",
    label: "Position",
    labelPlural: "Positions",
    module: "membership",
    blurb: "Unit-owned billet that exists while vacant — not just a link end.",
    get: (id) => `/membership/v1/positions/${id}`,
    title: (p) => s(p.code) || ridTail(p.id),
    subtitle: (p) => loc(p.title) || undefined,
    columns: [
      { key: "code", header: "Code", value: (p) => s(p.code), render: "mono" },
      { key: "title", header: "Title", value: (p) => loc(p.title) },
      { key: "status", header: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
    ],
    properties: [
      { label: "Code", value: (p) => s(p.code), render: "mono" },
      { label: "Title", value: (p) => loc(p.title) },
      { label: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
      { label: "Unit", value: (p) => s(p.unitId), render: "mono" },
    ],
  },

  document: {
    type: "document",
    kind: "object",
    label: "Document",
    labelPlural: "Documents",
    module: "document",
    blurb: "Person-held identity paper; catalog-typed, metadata only.",
    list: { path: "/document/v1/documents", search: "?pageSize=50", parse: pageParse("documents") },
    get: (id) => `/document/v1/documents/${id}`,
    title: (d) => s(d.number) || ridTail(d.id),
    subtitle: (d) => s(d.status),
    columns: [
      { key: "number", header: "Number", value: (d) => s(d.number), render: "mono" },
      { key: "typeId", header: "Document type", value: (d) => (s(d.typeId) ? ridTail(s(d.typeId)!) : undefined), render: "mono" },
      { key: "issuer", header: "Issuer", value: (d) => s(d.issuer) },
      { key: "issuedOn", header: "Issued on", value: (d) => s(d.issuedOn) },
      { key: "expiresOn", header: "Expires on", value: (d) => s(d.expiresOn) },
      { key: "status", header: "Status", value: (d) => s(d.status), render: "pill", tone: (d) => statusTone(d.status) },
    ],
    filters: [
      { key: "typeId", kind: "ref", label: "Document type", params: ["typeId"], control: "documentType" },
      {
        key: "status", kind: "enum", label: "Status", params: ["status"],
        values: [
          { value: "active", label: "Active" },
          { value: "superseded", label: "Superseded" },
          { value: "revoked", label: "Revoked" },
        ],
      },
      { key: "issuingCountryId", kind: "ref", label: "Issuing country", params: ["issuingCountryId"], control: "country" },
      {
        key: "issuedOn", kind: "date-range", label: "Issued on",
        params: ["issuedOnFrom", "issuedOnTo"],
        buckets: "dateTrunc",
        hint: "Setting either bound excludes documents with no issue date.",
      },
      {
        key: "expiresOn", kind: "date-range", label: "Expires on",
        params: ["expiresOnFrom", "expiresOnTo"],
        buckets: "dateTrunc",
        hint: "Setting either bound excludes documents with no expiry.",
      },
    ],
    dashboard: {
      path: "/document/v1/stats/documents",
      charts: [
        {
          // Leads: it is the one component with an operational deadline attached.
          key: "expiringSoon", title: "Expiring within 90 days", form: "stat", facet: "expiresOn",
          derived: "expiringSoon",
          note: "An exact count over the window, not a month-boundary approximation.",
        },
        {
          // Past-due months are toned red by the dashboard, which knows today's date; a per-bucket
          // tone map cannot express "before now".
          key: "expiry", title: "Expiry by month", form: "histogram", facet: "expiresOn",
          pastDue: true,
          note: "Months already past are overdue documents, not a forecast.",
        },
        {
          key: "status", title: "Status", form: "donut", facet: "status",
          tone: { active: "green", superseded: "slate", revoked: "red" },
        },
        { key: "types", title: "Type mix", form: "bar", facet: "typeId", orientation: "horizontal" },
        {
          key: "issuingCountry", title: "Issuing country", form: "bar", facet: "issuingCountryId",
          orientation: "horizontal",
        },
      ],
    },
    properties: [
      { label: "Number", value: (d) => s(d.number), render: "mono" },
      { label: "Issuer", value: (d) => s(d.issuer) },
      { label: "Issuing country", value: (d) => s(d.issuingCountry) },
      { label: "Issued on", value: (d) => s(d.issuedOn) },
      { label: "Expires on", value: (d) => s(d.expiresOn) },
      { label: "Status", value: (d) => s(d.status), render: "pill", tone: (d) => statusTone(d.status) },
      { label: "Person", value: (d) => s(d.personId), render: "mono" },
    ],
  },

  // ── M13: social & messenger ────────────────────────────────────────────────
  social_account: {
    type: "social_account",
    kind: "object",
    label: "Social account",
    labelPlural: "Social accounts",
    module: "person",
    blurb: "A person's account on a social/messenger platform; handle is mutable (history kept).",
    // person-scoped (no standalone GET) — surfaced via the person detail manager & link rows.
    title: (a) => (s(a.handle) ? `@${s(a.handle)}` : ridTail(a.id)),
    subtitle: (a) => s(a.platformCode),
    columns: [
      { key: "handle", header: "Handle", value: (a) => `@${s(a.handle) ?? ""}` },
      { key: "platform", header: "Platform", value: (a) => s(a.platformCode) },
      { key: "source", header: "Source", value: (a) => s(a.source) },
      { key: "confidence", header: "Confidence", value: (a) => s(a.confidence) },
      { key: "verified", header: "Verified", value: (a) => (a.platformVerified ? "platform" : ""), render: "pill", tone: (a) => (a.platformVerified ? "green" : "slate") },
    ],
    properties: [
      { label: "Handle", value: (a) => `@${s(a.handle) ?? ""}` },
      { label: "Platform", value: (a) => s(a.platformCode) },
      { label: "Display name", value: (a) => s(a.displayName) },
      { label: "Profile URL", value: (a) => s(a.profileUrl) },
      { label: "Source", value: (a) => s(a.source) },
      { label: "Confidence", value: (a) => s(a.confidence) },
      { label: "Platform-verified", value: (a) => (a.platformVerified ? "yes" : "no") },
      { label: "Person", value: (a) => s(a.personId), render: "mono" },
    ],
  },

  messenger_link: {
    type: "messenger_link",
    kind: "link",
    label: "Messenger link",
    labelPlural: "Messenger links",
    module: "person",
    blurb: "Reachability over a person's phone XOR email on a messenger platform (link__reachable_on).",
    title: (l) => `${s(l.platformCode) ?? "messenger"} ${ridTail(l.id)}`,
    subtitle: (l) => s(l.platformCode),
    columns: [
      { key: "platform", header: "Platform", value: (l) => s(l.platformCode) },
      { key: "channel", header: "Channel", value: (l) => (l.phoneId ? "phone" : l.emailId ? "email" : "—") },
      { key: "primary", header: "Primary", value: (l) => (l.isPrimary ? "primary" : ""), render: "pill", tone: () => "indigo" },
    ],
    properties: [
      { label: "Platform", value: (l) => s(l.platformCode) },
      { label: "Phone", value: (l) => s(l.phoneId), render: "mono" },
      { label: "Email", value: (l) => s(l.emailId), render: "mono" },
      { label: "Verified at", value: (l) => s(l.verifiedAt) },
    ],
  },

  platform: {
    type: "platform",
    kind: "object",
    label: "Platform",
    labelPlural: "Platforms",
    module: "person",
    blurb: "Instance-admin catalog of messenger/social platforms; category gates which links are allowed.",
    list: { path: "/person/v1/person/platforms", parse: catalogParse },
    title: (p) => s(p.code) || ridTail(p.id),
    subtitle: (p) => loc(p.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (p) => s(p.code), render: "mono" },
      { key: "name", header: "Name", value: (p) => loc(p.name) },
      { key: "category", header: "Category", value: (p) => s(p.category), render: "pill", tone: (p) => (p.category === "messenger" ? "indigo" : "slate") },
      { key: "status", header: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
    ],
  },

  // ── M14: person↔person relationship catalog ─────────────────────────────────
  relation_type: {
    type: "relation_type",
    kind: "object",
    label: "Relation type",
    labelPlural: "Relation types",
    module: "person",
    blurb: "Instance-admin catalog of relation kinds; category scopes which relationship family uses it.",
    list: { path: "/person/v1/person/relation-types", parse: catalogParse },
    title: (t) => s(t.code) || ridTail(t.id),
    subtitle: (t) => loc(t.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (t) => s(t.code), render: "mono" },
      { key: "name", header: "Name", value: (t) => loc(t.name) },
      { key: "category", header: "Category", value: (t) => s(t.category) },
      { key: "status", header: "Status", value: (t) => s(t.status), render: "pill", tone: (t) => statusTone(t.status) },
    ],
  },

  // ── M15: rank systems ───────────────────────────────────────────────────────
  system: {
    type: "system",
    kind: "object",
    label: "Rank system",
    labelPlural: "Rank systems",
    module: "rank",
    blurb: "Top level of the rank scheme (e.g. us-armed-forces, nato-generic); national or supranational.",
    // no standalone GET — managed via the Rank scheme page; registered for badges/graph typing.
    title: (sys) => s(sys.code) || ridTail(sys.id),
    subtitle: (sys) => loc(sys.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (sys) => s(sys.code), render: "mono" },
      { key: "name", header: "Name", value: (sys) => loc(sys.name) },
      { key: "country", header: "Country", value: (sys) => s(sys.country) || "supranational" },
    ],
  },

  rank: {
    type: "rank",
    kind: "object",
    label: "Rank",
    labelPlural: "Ranks",
    module: "rank",
    blurb: "A single rank within a leaf type; may carry a NATO STANAG-2116 grade code for equivalence.",
    title: (r) => loc(r.name) || s(r.code) || ridTail(r.id),
    subtitle: (r) => s(r.abbreviation) || s(r.code),
    columns: [
      { key: "code", header: "Code", value: (r) => s(r.code), render: "mono" },
      { key: "name", header: "Name", value: (r) => loc(r.name) },
      { key: "abbr", header: "Abbr", value: (r) => s(r.abbreviation) },
      { key: "grade", header: "Grade", value: (r) => s(r.gradeCode), render: "pill", tone: () => "indigo" },
    ],
    properties: [
      { label: "Code", value: (r) => s(r.code), render: "mono" },
      { label: "Name", value: (r) => loc(r.name) },
      { label: "Abbreviation", value: (r) => s(r.abbreviation) },
      { label: "Grade (STANAG)", value: (r) => s(r.gradeCode) },
    ],
  },

  languoid: {
    type: "languoid",
    kind: "object",
    label: "Language",
    labelPlural: "Languages",
    module: "language",
    blurb: "A Glottolog languoid (family/language/dialect), keyed by glottocode; the genealogical catalog.",
    list: { path: "/language/v1/languages", search: "?limit=100", searchParam: "query", parse: pageParse("languoids") },
    get: (id) => `/language/v1/languages/${id}`,
    title: (l) => loc(l.name) || s(l.code) || ridTail(l.id),
    subtitle: (l) => s(l.code),
    columns: [
      { key: "code", header: "Glottocode", value: (l) => s(l.code), render: "mono" },
      { key: "name", header: "Name", value: (l) => loc(l.name) },
      { key: "level", header: "Level", value: (l) => s(l.level), render: "pill", tone: (l) => (s(l.level) === "family" ? "indigo" : s(l.level) === "language" ? "green" : "slate") },
      { key: "iso", header: "ISO 639-3", value: (l) => s(l.iso6393), render: "mono" },
      { key: "family", header: "Family", value: (l) => s(l.familyCode), render: "mono" },
    ],
    properties: [
      { label: "Glottocode", value: (l) => s(l.code), render: "mono" },
      { label: "Name", value: (l) => loc(l.name) },
      { label: "Level", value: (l) => s(l.level), render: "pill" },
      { label: "ISO 639-3", value: (l) => s(l.iso6393), render: "mono" },
      { label: "Family", value: (l) => s(l.familyCode), render: "mono" },
      { label: "Macroarea", value: (l) => s(l.macroarea) },
      { label: "Status", value: (l) => s(l.status) },
    ],
  },

  writing_system: {
    type: "writing_system",
    kind: "object",
    label: "Writing system",
    labelPlural: "Writing systems",
    module: "language",
    blurb: "An ISO-15924 script (e.g. Latn, Cyrl), classified by script type; the writing-system catalog.",
    list: { path: "/language/v1/writing-systems", parse: pageParse("writingSystems") },
    title: (w) => loc(w.name) || s(w.code) || ridTail(w.id),
    subtitle: (w) => s(w.code),
    columns: [
      { key: "code", header: "Code", value: (w) => s(w.code), render: "mono" },
      { key: "name", header: "Name", value: (w) => loc(w.name) },
      { key: "scriptType", header: "Script type", value: (w) => s(w.scriptType), render: "pill", tone: () => "indigo" },
    ],
  },

  // ── M19: location ────────────────────────────────────────────────────────────
  location: {
    type: "location",
    kind: "object",
    label: "Location",
    labelPlural: "Locations",
    module: "location",
    blurb: "A shared, standalone place: a coordinate + an app-derived MGRS + a structured address. Browse from the Locations page (a spatial query is required).",
    // No unconditional list (listLocations needs a radius/bbox window) — get/properties so the object
    // view & graph badges work; the Locations page is the browse/create surface.
    get: (id) => `/location/v1/locations/${id}`,
    title: (l) => s(l.mgrs) || s(l.locality) || ridTail(l.id),
    subtitle: (l) => (l.latitude != null && l.longitude != null ? `${s(l.latitude)}, ${s(l.longitude)}` : undefined),
    columns: [
      { key: "mgrs", header: "MGRS", value: (l) => s(l.mgrs), render: "mono" },
      { key: "locality", header: "Locality", value: (l) => s(l.locality) },
      { key: "lat", header: "Lat", value: (l) => s(l.latitude), render: "mono", align: "right" },
      { key: "lng", header: "Lng", value: (l) => s(l.longitude), render: "mono", align: "right" },
    ],
    properties: [
      { label: "Latitude", value: (l) => s(l.latitude), render: "mono" },
      { label: "Longitude", value: (l) => s(l.longitude), render: "mono" },
      { label: "MGRS", value: (l) => s(l.mgrs), render: "mono" },
      { label: "Source format", value: (l) => s((l.sourceCoordinate as { format?: string } | undefined)?.format), render: "mono" },
      { label: "Type", value: (l) => loc(l.typeName) || undefined },
      { label: "Country", value: (l) => (l.countryId ? ridTail(s(l.countryId)!) : undefined), render: "mono" },
      { label: "Locality", value: (l) => s(l.locality) },
      { label: "Street", value: (l) => [s(l.street), s(l.houseNumber)].filter(Boolean).join(" ") || undefined },
      { label: "Admin area", value: (l) => [s(l.adminArea1), s(l.adminArea2)].filter(Boolean).join(", ") || undefined },
      { label: "Postal code", value: (l) => s(l.postalCode) },
      { label: "Raw address", value: (l) => s(l.rawAddress) },
    ],
  },

  location_type: {
    type: "location_type",
    kind: "object",
    label: "Location type",
    labelPlural: "Location types",
    module: "location",
    blurb: "Instance-admin catalog of place purposes (building/address/online); descriptive only.",
    list: { path: "/location/v1/location/types", parse: pageParse("locationTypes") },
    title: (t) => s(t.code) || ridTail(t.id),
    subtitle: (t) => loc(t.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (t) => s(t.code), render: "mono" },
      { key: "name", header: "Name", value: (t) => loc(t.name) },
      { key: "status", header: "Status", value: (t) => s(t.status), render: "pill", tone: (t) => statusTone(t.status) },
    ],
  },

  // Education (M20 / D-Education): external reference institutions + their structure tree, buildings,
  // groups and positions. Institutions are globally browsable; the children are reached per institution.
  institution: {
    type: "institution",
    kind: "object",
    label: "Institution",
    labelPlural: "Institutions",
    module: "education",
    blurb: "External reference education institution (where people studied/taught) + its structure tree.",
    list: { path: "/education/v1/institutions", search: "?pageSize=50", parse: pageParse("institutions") },
    get: (id) => `/education/v1/institutions/${id}`,
    title: (i) => loc(i.name) || s(i.code) || ridTail(i.id),
    subtitle: (i) => s(i.code),
    columns: [
      { key: "code", header: "Code", value: (i) => s(i.code), render: "mono" },
      { key: "name", header: "Name", value: (i) => loc(i.name) },
      { key: "state", header: "State", value: (i) => s(i.state), render: "pill", tone: (i) => statusTone(i.state) },
    ],
    properties: [
      { label: "Name", value: (i) => loc(i.name) },
      { label: "Code", value: (i) => s(i.code), render: "mono" },
      { label: "Kind", value: (i) => (i.kindId ? ridTail(s(i.kindId)!) : undefined), render: "mono" },
      { label: "Country", value: (i) => (i.countryId ? ridTail(s(i.countryId)!) : undefined), render: "mono" },
      { label: "Founded", value: (i) => s(i.foundedOn) },
      { label: "Closed", value: (i) => s(i.closedOn) },
      { label: "State", value: (i) => s(i.state), render: "pill", tone: (i) => statusTone(i.state) },
    ],
    links: [
      {
        label: "Units",
        targetType: "education_unit",
        path: (id) => `/education/v1/institutions/${id}/units`,
        parse: (r) => arr(r, "units").map((u) => ({ id: s(u.id)!, label: loc(u.name) || s(u.code) || ridTail(s(u.id)!), sub: `depth ${s(u.depth) ?? "0"}` })),
      },
      {
        label: "Buildings",
        targetType: "building",
        path: (id) => `/education/v1/institutions/${id}/buildings`,
        parse: (r) => arr(r, "buildings").map((b) => ({ id: s(b.id)!, label: loc(b.name) || s(b.code) || ridTail(s(b.id)!), sub: s(b.kind) })),
      },
      {
        label: "Positions",
        targetType: "education_position",
        path: (id) => `/education/v1/institutions/${id}/positions`,
        parse: (r) => arr(r, "positions").map((p) => ({ id: s(p.id)!, label: loc(p.title) || s(p.code) || ridTail(s(p.id)!), sub: p.holder ? "filled" : "vacant", tone: p.holder ? "green" : "amber" })),
      },
      {
        label: "Programs",
        targetType: "program",
        path: (id) => `/education/v1/institutions/${id}/programs`,
        parse: (r) => arr(r, "programs").map((p) => ({ id: s(p.id)!, label: s(p.name) || s(p.code) || ridTail(s(p.id)!), sub: s(p.mode) })),
      },
      {
        label: "Courses",
        targetType: "course",
        path: (id) => `/education/v1/institutions/${id}/courses`,
        parse: (r) => arr(r, "courses").map((c) => ({ id: s(c.id)!, label: s(c.title) || s(c.code) || ridTail(s(c.id)!), sub: s(c.code) })),
      },
      {
        label: "Research centres",
        targetType: "research_centre",
        path: (id) => `/education/v1/institutions/${id}/research-centres`,
        parse: (r) => arr(r, "researchCentres").map((c) => ({ id: s(c.id)!, label: s(c.name) || s(c.code) || ridTail(s(c.id)!), sub: s(c.kind) })),
      },
      {
        label: "Grants",
        targetType: "grant",
        path: (id) => `/education/v1/institutions/${id}/grants`,
        parse: (r) => arr(r, "grants").map((g) => ({ id: s(g.id)!, label: s(g.title) || s(g.code) || ridTail(s(g.id)!), sub: s(g.status) })),
      },
      {
        label: "Governance bodies",
        targetType: "governance_body",
        path: (id) => `/education/v1/institutions/${id}/governance-bodies`,
        parse: (r) => arr(r, "governanceBodies").map((b) => ({ id: s(b.id)!, label: s(b.name) || s(b.code) || ridTail(s(b.id)!), sub: s(b.kind) })),
      },
      {
        label: "Qualifications",
        targetType: "qualification",
        path: (id) => `/education/v1/institutions/${id}/qualifications`,
        parse: (r) => arr(r, "qualifications").map((q) => ({ id: s(q.id)!, label: s(q.name) || s(q.code) || ridTail(s(q.id)!), sub: s(q.frameworkCode) })),
      },
    ],
  },

  education_unit: {
    type: "education_unit",
    kind: "object",
    label: "Education unit",
    labelPlural: "Education units",
    module: "education",
    blurb: "A node in an institution's recursive structure tree (campus/faculty/department/chair).",
    get: (id) => `/education/v1/units/${id}`,
    title: (u) => loc(u.name) || s(u.code) || ridTail(u.id),
    subtitle: (u) => s(u.code),
    columns: [
      { key: "code", header: "Code", value: (u) => s(u.code), render: "mono" },
      { key: "name", header: "Name", value: (u) => loc(u.name) },
      { key: "status", header: "Status", value: (u) => s(u.status), render: "pill", tone: (u) => statusTone(u.status) },
    ],
    properties: [
      { label: "Name", value: (u) => loc(u.name) },
      { label: "Code", value: (u) => s(u.code), render: "mono" },
      { label: "Institution", value: (u) => (u.institutionId ? ridTail(s(u.institutionId)!) : undefined), render: "mono" },
      { label: "Parent", value: (u) => (u.parentId ? ridTail(s(u.parentId)!) : undefined), render: "mono" },
      { label: "Status", value: (u) => s(u.status), render: "pill", tone: (u) => statusTone(u.status) },
    ],
    links: [
      {
        label: "Groups",
        targetType: "group",
        path: (id) => `/education/v1/units/${id}/groups`,
        parse: (r) => arr(r, "groups").map((g) => ({ id: s(g.id)!, label: loc(g.name) || s(g.code) || ridTail(s(g.id)!), sub: s(g.admissionYear) })),
      },
    ],
  },

  building: {
    type: "building",
    kind: "object",
    label: "Building",
    labelPlural: "Buildings",
    module: "education",
    blurb: "A building of an institution (academic/dormitory/…), located via the shared Location.",
    get: (id) => `/education/v1/buildings/${id}`,
    title: (b) => loc(b.name) || s(b.code) || ridTail(b.id),
    subtitle: (b) => s(b.kind),
    columns: [
      { key: "code", header: "Code", value: (b) => s(b.code), render: "mono" },
      { key: "name", header: "Name", value: (b) => loc(b.name) },
      { key: "kind", header: "Kind", value: (b) => s(b.kind), render: "pill" },
    ],
    properties: [
      { label: "Name", value: (b) => loc(b.name) },
      { label: "Code", value: (b) => s(b.code), render: "mono" },
      { label: "Kind", value: (b) => s(b.kind), render: "pill" },
      { label: "Institution", value: (b) => (b.institutionId ? ridTail(s(b.institutionId)!) : undefined), render: "mono" },
      { label: "Location", value: (b) => (b.locationId ? ridTail(s(b.locationId)!) : undefined), render: "mono" },
    ],
  },

  group: {
    type: "group",
    kind: "object",
    label: "Study group",
    labelPlural: "Study groups",
    module: "education",
    blurb: "A cohort under an education unit, with an admission year.",
    get: (id) => `/education/v1/groups/${id}`,
    title: (g) => loc(g.name) || s(g.code) || ridTail(g.id),
    subtitle: (g) => s(g.code),
    columns: [
      { key: "code", header: "Code", value: (g) => s(g.code), render: "mono" },
      { key: "name", header: "Name", value: (g) => loc(g.name) },
      { key: "admissionYear", header: "Year", value: (g) => s(g.admissionYear) },
    ],
    properties: [
      { label: "Name", value: (g) => loc(g.name) },
      { label: "Code", value: (g) => s(g.code), render: "mono" },
      { label: "Admission year", value: (g) => s(g.admissionYear) },
      { label: "Status", value: (g) => s(g.status), render: "pill", tone: (g) => statusTone(g.status) },
    ],
  },

  education_position: {
    type: "education_position",
    kind: "object",
    label: "Education position",
    labelPlural: "Education positions",
    module: "education",
    blurb: "An institution/unit-owned billet (rector/dean/chair); vacant-first, one active holder.",
    get: (id) => `/education/v1/positions/${id}`,
    title: (p) => loc(p.title) || s(p.code) || ridTail(p.id),
    subtitle: (p) => s(p.code),
    columns: [
      { key: "code", header: "Code", value: (p) => s(p.code), render: "mono" },
      { key: "title", header: "Title", value: (p) => loc(p.title) },
      { key: "status", header: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
    ],
    properties: [
      { label: "Title", value: (p) => loc(p.title) },
      { label: "Code", value: (p) => s(p.code), render: "mono" },
      { label: "Institution", value: (p) => (p.institutionId ? ridTail(s(p.institutionId)!) : undefined), render: "mono" },
      { label: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
      { label: "Holder", value: (p) => { const h = p.holder as { personId?: string } | undefined; return h?.personId ? ridTail(h.personId) : "vacant"; }, render: "mono" },
    ],
  },

  // Education reference layer (M20 extension). Reference entities use plain-string names/titles.
  program: {
    type: "program", kind: "object", label: "Program", labelPlural: "Programs", module: "education",
    blurb: "A degree/diploma/certificate program of an institution.",
    get: (id) => `/education/v1/programs/${id}`,
    title: (p) => s(p.name) || s(p.code) || ridTail(p.id), subtitle: (p) => s(p.code),
    columns: [
      { key: "code", header: "Code", value: (p) => s(p.code), render: "mono" },
      { key: "name", header: "Name", value: (p) => s(p.name) },
      { key: "mode", header: "Mode", value: (p) => s(p.mode), render: "pill" },
    ],
    properties: [
      { label: "Name", value: (p) => s(p.name) },
      { label: "Code", value: (p) => s(p.code), render: "mono" },
      { label: "Mode", value: (p) => s(p.mode), render: "pill" },
      { label: "Duration (years)", value: (p) => s(p.durationYears) },
      { label: "Credit hours", value: (p) => s(p.creditHoursTotal) },
      { label: "Institution", value: (p) => (p.institutionId ? ridTail(s(p.institutionId)!) : undefined), render: "mono" },
      { label: "State", value: (p) => s(p.state), render: "pill", tone: (p) => statusTone(p.state) },
    ],
    links: [
      {
        label: "Curriculum versions", targetType: "curriculum_version",
        path: (id) => `/education/v1/programs/${id}/curriculum-versions`,
        parse: (r) => arr(r, "versions").map((v) => ({ id: s(v.id)!, label: s(v.versionCode) || ridTail(s(v.id)!), sub: s(v.status) })),
      },
    ],
  },

  course: {
    type: "course", kind: "object", label: "Course", labelPlural: "Courses", module: "education",
    blurb: "A unit of study / module / subject of an institution.",
    get: (id) => `/education/v1/courses/${id}`,
    title: (c) => s(c.title) || s(c.code) || ridTail(c.id), subtitle: (c) => s(c.code),
    columns: [
      { key: "code", header: "Code", value: (c) => s(c.code), render: "mono" },
      { key: "title", header: "Title", value: (c) => s(c.title) },
      { key: "deliveryMode", header: "Delivery", value: (c) => s(c.deliveryMode), render: "pill" },
    ],
    properties: [
      { label: "Title", value: (c) => s(c.title) },
      { label: "Code", value: (c) => s(c.code), render: "mono" },
      { label: "Credit hours", value: (c) => s(c.creditHours) },
      { label: "Level", value: (c) => s(c.level) },
      { label: "Delivery mode", value: (c) => s(c.deliveryMode), render: "pill" },
      { label: "Institution", value: (c) => (c.institutionId ? ridTail(s(c.institutionId)!) : undefined), render: "mono" },
    ],
    links: [
      {
        label: "Prerequisites", targetType: "course",
        path: (id) => `/education/v1/courses/${id}/prerequisites`,
        parse: (r) => arr(r, "prerequisites").map((p) => ({ id: s(p.requiredCourseId)!, label: ridTail(s(p.requiredCourseId)!), sub: s(p.kind) })),
      },
    ],
  },

  curriculum_version: {
    type: "curriculum_version", kind: "object", label: "Curriculum version", labelPlural: "Curriculum versions", module: "education",
    blurb: "A versioned snapshot of a program's course requirements.",
    get: (id) => `/education/v1/curriculum-versions/${id}`,
    title: (v) => s(v.versionCode) || ridTail(v.id), subtitle: (v) => s(v.status),
    columns: [
      { key: "versionCode", header: "Version", value: (v) => s(v.versionCode), render: "mono" },
      { key: "status", header: "Status", value: (v) => s(v.status), render: "pill", tone: (v) => statusTone(v.status) },
    ],
    properties: [
      { label: "Version", value: (v) => s(v.versionCode), render: "mono" },
      { label: "Program", value: (v) => (v.programId ? ridTail(s(v.programId)!) : undefined), render: "mono" },
      { label: "Effective from", value: (v) => s(v.effectiveFrom) },
      { label: "Status", value: (v) => s(v.status), render: "pill", tone: (v) => statusTone(v.status) },
    ],
    links: [
      {
        label: "Items", targetType: "course",
        path: (id) => `/education/v1/curriculum-versions/${id}/items`,
        parse: (r) => arr(r, "items").map((it) => ({ id: s(it.courseId)!, label: ridTail(s(it.courseId)!), sub: it.isRequired ? "required" : "elective" })),
      },
    ],
  },

  research_centre: {
    type: "research_centre", kind: "object", label: "Research centre", labelPlural: "Research centres", module: "education",
    blurb: "A research centre / institute / lab of an institution.",
    get: (id) => `/education/v1/research-centres/${id}`,
    title: (c) => s(c.name) || s(c.code) || ridTail(c.id), subtitle: (c) => s(c.code),
    columns: [
      { key: "code", header: "Code", value: (c) => s(c.code), render: "mono" },
      { key: "name", header: "Name", value: (c) => s(c.name) },
      { key: "kind", header: "Kind", value: (c) => s(c.kind), render: "pill" },
    ],
    properties: [
      { label: "Name", value: (c) => s(c.name) },
      { label: "Code", value: (c) => s(c.code), render: "mono" },
      { label: "Kind", value: (c) => s(c.kind), render: "pill" },
      { label: "Funding source", value: (c) => s(c.fundingSource) },
      { label: "Status", value: (c) => s(c.status), render: "pill", tone: (c) => statusTone(c.status) },
    ],
  },

  research_group: {
    type: "research_group", kind: "object", label: "Research group", labelPlural: "Research groups", module: "education",
    blurb: "A research cluster under a centre and/or unit.",
    get: (id) => `/education/v1/research-groups/${id}`,
    title: (g) => s(g.name) || s(g.code) || ridTail(g.id), subtitle: (g) => s(g.code),
    columns: [
      { key: "code", header: "Code", value: (g) => s(g.code), render: "mono" },
      { key: "name", header: "Name", value: (g) => s(g.name) },
      { key: "status", header: "Status", value: (g) => s(g.status), render: "pill", tone: (g) => statusTone(g.status) },
    ],
    properties: [
      { label: "Name", value: (g) => s(g.name) },
      { label: "Code", value: (g) => s(g.code), render: "mono" },
      { label: "Focus area", value: (g) => s(g.focusArea) },
      { label: "Status", value: (g) => s(g.status), render: "pill", tone: (g) => statusTone(g.status) },
    ],
  },

  grant: {
    type: "grant", kind: "object", label: "Grant", labelPlural: "Grants", module: "education",
    blurb: "A funding grant held by an institution.",
    get: (id) => `/education/v1/grants/${id}`,
    title: (g) => s(g.title) || s(g.code) || ridTail(g.id), subtitle: (g) => s(g.code),
    columns: [
      { key: "code", header: "Code", value: (g) => s(g.code), render: "mono" },
      { key: "title", header: "Title", value: (g) => s(g.title) },
      { key: "status", header: "Status", value: (g) => s(g.status), render: "pill", tone: (g) => statusTone(g.status) },
    ],
    properties: [
      { label: "Title", value: (g) => s(g.title) },
      { label: "Code", value: (g) => s(g.code), render: "mono" },
      { label: "Funder", value: (g) => s(g.funder) },
      { label: "Amount", value: (g) => { const a = s(g.amount); const c = s(g.currency); return a ? `${a} ${c ?? ""}`.trim() : undefined; } },
      { label: "Status", value: (g) => s(g.status), render: "pill", tone: (g) => statusTone(g.status) },
    ],
  },

  publication: {
    type: "publication", kind: "object", label: "Publication", labelPlural: "Publications", module: "education",
    blurb: "An academic publication (optionally tied to an institution).",
    list: { path: "/education/v1/publications", search: "", parse: (r) => ({ rows: arr(r, "publications") as Row[] }) },
    get: (id) => `/education/v1/publications/${id}`,
    title: (p) => s(p.title) || s(p.code) || ridTail(p.id), subtitle: (p) => s(p.code),
    columns: [
      { key: "code", header: "Code", value: (p) => s(p.code), render: "mono" },
      { key: "title", header: "Title", value: (p) => s(p.title) },
      { key: "kind", header: "Kind", value: (p) => s(p.kind), render: "pill" },
    ],
    properties: [
      { label: "Title", value: (p) => s(p.title) },
      { label: "Code", value: (p) => s(p.code), render: "mono" },
      { label: "Kind", value: (p) => s(p.kind), render: "pill" },
      { label: "DOI", value: (p) => s(p.doi), render: "mono" },
      { label: "Venue", value: (p) => s(p.venue) },
      { label: "Published", value: (p) => s(p.publishedOn) },
    ],
  },

  governance_body: {
    type: "governance_body", kind: "object", label: "Governance body", labelPlural: "Governance bodies", module: "education",
    blurb: "A board / senate / council / committee of an institution.",
    get: (id) => `/education/v1/governance-bodies/${id}`,
    title: (b) => s(b.name) || s(b.code) || ridTail(b.id), subtitle: (b) => s(b.code),
    columns: [
      { key: "code", header: "Code", value: (b) => s(b.code), render: "mono" },
      { key: "name", header: "Name", value: (b) => s(b.name) },
      { key: "kind", header: "Kind", value: (b) => s(b.kind), render: "pill" },
    ],
    properties: [
      { label: "Name", value: (b) => s(b.name) },
      { label: "Code", value: (b) => s(b.code), render: "mono" },
      { label: "Kind", value: (b) => s(b.kind), render: "pill" },
      { label: "Mandate", value: (b) => s(b.mandate) },
      { label: "Status", value: (b) => s(b.status), render: "pill", tone: (b) => statusTone(b.status) },
    ],
  },

  policy: {
    type: "policy", kind: "object", label: "Policy", labelPlural: "Policies", module: "education",
    blurb: "An institutional rule/regulation.",
    get: (id) => `/education/v1/policies/${id}`,
    title: (p) => s(p.title) || s(p.code) || ridTail(p.id), subtitle: (p) => s(p.code),
    columns: [
      { key: "code", header: "Code", value: (p) => s(p.code), render: "mono" },
      { key: "title", header: "Title", value: (p) => s(p.title) },
      { key: "status", header: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
    ],
    properties: [
      { label: "Title", value: (p) => s(p.title) },
      { label: "Code", value: (p) => s(p.code), render: "mono" },
      { label: "Kind", value: (p) => s(p.kind), render: "pill" },
      { label: "Effective", value: (p) => s(p.effectiveOn) },
      { label: "Status", value: (p) => s(p.status), render: "pill", tone: (p) => statusTone(p.status) },
    ],
  },

  qualification: {
    type: "qualification", kind: "object", label: "Qualification", labelPlural: "Qualifications", module: "education",
    blurb: "A formally awardable qualification (degree) classification.",
    get: (id) => `/education/v1/qualifications/${id}`,
    title: (q) => s(q.name) || s(q.code) || ridTail(q.id), subtitle: (q) => s(q.code),
    columns: [
      { key: "code", header: "Code", value: (q) => s(q.code), render: "mono" },
      { key: "name", header: "Name", value: (q) => s(q.name) },
      { key: "frameworkCode", header: "Framework", value: (q) => s(q.frameworkCode), render: "pill" },
    ],
    properties: [
      { label: "Name", value: (q) => s(q.name) },
      { label: "Code", value: (q) => s(q.code), render: "mono" },
      { label: "Framework", value: (q) => s(q.frameworkCode), render: "pill" },
      { label: "Framework level", value: (q) => s(q.frameworkLevel) },
      { label: "Awarding body", value: (q) => s(q.awardingBody) },
      { label: "Status", value: (q) => s(q.status), render: "pill", tone: (q) => statusTone(q.status) },
    ],
  },

  scholarship: {
    type: "scholarship", kind: "object", label: "Scholarship", labelPlural: "Scholarships", module: "education",
    blurb: "A financial award scheme (institution or external).",
    list: { path: "/education/v1/scholarships", search: "", parse: (r) => ({ rows: arr(r, "scholarships") as Row[] }) },
    get: (id) => `/education/v1/scholarships/${id}`,
    title: (x) => s(x.name) || s(x.code) || ridTail(x.id), subtitle: (x) => s(x.code),
    columns: [
      { key: "code", header: "Code", value: (x) => s(x.code), render: "mono" },
      { key: "name", header: "Name", value: (x) => s(x.name) },
      { key: "kind", header: "Kind", value: (x) => s(x.kind), render: "pill" },
    ],
    properties: [
      { label: "Name", value: (x) => s(x.name) },
      { label: "Code", value: (x) => s(x.code), render: "mono" },
      { label: "Kind", value: (x) => s(x.kind), render: "pill" },
      { label: "Amount", value: (x) => { const a = s(x.amount); const c = s(x.currency); return a ? `${a} ${c ?? ""}`.trim() : undefined; } },
      { label: "Frequency", value: (x) => s(x.frequency), render: "pill" },
      { label: "Status", value: (x) => s(x.status), render: "pill", tone: (x) => statusTone(x.status) },
    ],
  },

  accreditation_event: {
    type: "accreditation_event", kind: "object", label: "Accreditation event", labelPlural: "Accreditation events", module: "education",
    blurb: "An accreditation review cycle against an institution or program.",
    get: (id) => `/education/v1/accreditation-events/${id}`,
    title: (e) => `${s(e.outcome) ?? "event"} (${s(e.entityKind) ?? ""})`, subtitle: (e) => s(e.reviewOn),
    columns: [
      { key: "entityKind", header: "Target", value: (e) => s(e.entityKind), render: "pill" },
      { key: "outcome", header: "Outcome", value: (e) => s(e.outcome), render: "pill" },
      { key: "reviewOn", header: "Reviewed", value: (e) => s(e.reviewOn) },
    ],
    properties: [
      { label: "Target", value: (e) => s(e.entityKind), render: "pill" },
      { label: "Outcome", value: (e) => s(e.outcome), render: "pill" },
      { label: "Body", value: (e) => s(e.body) },
      { label: "Reviewed", value: (e) => s(e.reviewOn) },
      { label: "Effective from", value: (e) => s(e.effectiveFrom) },
    ],
  },
};

/**
 * Read permission code per module (D-SelfCapabilities). Most types are gated by their module's
 * `*.read`; the few types whose read code differs from the module default carry an explicit
 * `requires` on the type def and win over this map (see readCodeFor). Codes mirror
 * internal/authorization/domain/permissions.go — keep in sync.
 */
const READ_CODE_BY_MODULE: Record<string, string> = {
  person: "person.read",
  tenant: "unit.read",
  order: "order.read",
  authorization: "role.read",
  document: "document.read",
  localization: "locale.read",
  membership: "membership.read",
  rank: "rank.scheme.read",
  language: "language.read",
  location: "location.read",
  education: "education.read",
};

/**
 * The permission code a caller must hold (anywhere) to see this type in the console. Explicit
 * `def.requires` wins; otherwise the module default. Undefined means "no code known" — treated as
 * always-shown by the gating helpers, matching the console's historical open-by-default behaviour.
 */
export function readCodeFor(def: ObjectTypeDef): string | undefined {
  return def.requires ?? READ_CODE_BY_MODULE[def.module];
}

/** Object types that can be browsed as a global table (and fanned out in search). */
export const EXPLORABLE_TYPES = Object.values(OBJECT_TYPES).filter((t) => t.list);

/** Look up a type def by RID entity_type token. */
export function typeDef(type: string | null | undefined): ObjectTypeDef | undefined {
  return type ? OBJECT_TYPES[type] : undefined;
}

/** A flattened search string for client-side filtering in the palette. */
export function rowSearchText(def: ObjectTypeDef, row: Row): string {
  return [def.title(row), def.subtitle?.(row), row.code, row.id]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}
