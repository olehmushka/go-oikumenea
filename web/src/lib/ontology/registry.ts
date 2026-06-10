// The ontology registry — one data-driven description of each Object/Link type that powers the whole
// workspace: the explorer table, the command-palette fan-out search, the universal object view, the
// link/graph traversal, and the ontology browser. It is the human-facing mirror of D-Ontology
// (docs/ontology-mapping.md) and is keyed by the RID entity_type token (see ./rid).
//
// Isomorphic on purpose: imports only pure helpers (pickLabel, rid) so it can be used from server
// components (explorer/object pages) AND the browser bundle (palette/drawer). It carries NO JSX —
// it exposes string/number accessors + render hints, and the components do the rendering. This also
// keeps it serialization-safe: the server calls these accessors and passes plain data to clients.

import { pickLabel, type LocaleMap } from "@/lib/i18n";
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
  parse: (res: unknown) => { rows: Row[]; nextPageToken?: string };
}

export interface ObjectTypeDef {
  type: string;
  kind: "object" | "link" | "action";
  label: string;
  labelPlural: string;
  /** module the type belongs to (for the ontology browser grouping) */
  module: string;
  /** one-line description for the ontology browser */
  blurb?: string;
  /** present only for types with an unconditional top-level list (explorable + searchable) */
  list?: ListDef;
  /** single-object fetch for the object view/drawer */
  get?: (id: string) => string;
  title: (obj: Row) => string;
  subtitle?: (obj: Row) => string | undefined;
  columns: ColumnDef[];
  properties?: PropertyDef[];
  links?: LinkDef[];
  actions?: ActionDef[];
}

// ── small accessors ─────────────────────────────────────────────────────────
const s = (v: unknown): string | undefined => (v == null ? undefined : String(v));
const loc = (v: unknown): string => pickLabel(v as LocaleMap);
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
    list: { path: "/person/v1/persons", search: "?pageSize=50", parse: pageParse("persons") },
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
    list: { path: "/tenant/v1/units", search: "?pageSize=50", parse: pageParse("units") },
    get: (id) => `/tenant/v1/units/${id}`,
    title: (u) => s(u.code) || ridTail(u.id),
    subtitle: (u) => loc(u.name) || undefined,
    columns: [
      { key: "code", header: "Code", value: (u) => s(u.code), render: "mono" },
      { key: "name", header: "Name", value: (u) => loc(u.name) },
      { key: "unitKind", header: "Kind", value: (u) => s(u.unitKind) },
      { key: "level", header: "Level", value: (u) => s(u.level) },
      { key: "visibility", header: "Visibility", value: (u) => s(u.visibility), render: "pill", tone: (u) => statusTone(u.visibility) },
      { key: "state", header: "State", value: (u) => s(u.state), render: "pill", tone: (u) => statusTone(u.state) },
    ],
    properties: [
      { label: "Name", value: (u) => loc(u.name) },
      { label: "Code", value: (u) => s(u.code), render: "mono" },
      { label: "Kind", value: (u) => s(u.unitKind) },
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
    get: (id) => `/order/v1/orders/${id}`,
    title: (o) => (s(o.number) ? `Order ${s(o.number)}` : `Order ${ridTail(o.id)}`),
    subtitle: (o) => s(o.status),
    columns: [
      { key: "number", header: "Number", value: (o) => s(o.number) || ridTail(o.id), render: "mono" },
      { key: "issuedOn", header: "Issued on", value: (o) => s(o.issuedOn) },
      { key: "items", header: "Items", value: (o) => (o.items as unknown[])?.length ?? 0 },
      { key: "status", header: "Status", value: (o) => s(o.status), render: "pill", tone: (o) => statusTone(o.status) },
    ],
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

  graph: {
    type: "graph",
    kind: "object",
    label: "Graph",
    labelPlural: "Graphs",
    module: "tenant",
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
    get: (id) => `/document/v1/documents/${id}`,
    title: (d) => s(d.number) || ridTail(d.id),
    subtitle: (d) => s(d.status),
    columns: [
      { key: "number", header: "Number", value: (d) => s(d.number), render: "mono" },
      { key: "issuer", header: "Issuer", value: (d) => s(d.issuer) },
      { key: "status", header: "Status", value: (d) => s(d.status), render: "pill", tone: (d) => statusTone(d.status) },
    ],
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
  rank_system: {
    type: "rank_system",
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
};

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
