// RID = the self-describing composed URN primary key every entity carries (D-ResourceIdentifiers):
//   urn:oikumenea:<service>:<env>:<entity_type>:<uuid>
// The entity_type slot encodes the ontology kind: a bare token for an Object, `link__<type>` for a
// reified Link, `action__<type>` for an Action. Because the type travels *in* the id, the console can
// route, label, and render any RID without a server lookup — the foundation of the object workspace.

export interface ParsedRid {
  service: string;
  env: string;
  /** entity_type slot verbatim, e.g. "person", "link__member_of", "action__issue_order" */
  type: string;
  uuid: string;
  /** ontology kind derived from the type prefix */
  kind: "object" | "link" | "action";
}

/** True for a string shaped like an oikumenea RID. Cheap guard before parseRid. */
export function isRid(s: string | null | undefined): s is string {
  return typeof s === "string" && s.startsWith("urn:oikumenea:") && s.split(":").length >= 6;
}

/** Parse a RID into its parts, or null if it isn't one. Never throws. */
export function parseRid(s: string | null | undefined): ParsedRid | null {
  if (!isRid(s)) return null;
  // urn:oikumenea:<service>:<env>:<entity_type>:<uuid> — split on the first 5 colons; the uuid is the
  // remainder (a uuid has no colons, but stay defensive and re-join).
  const parts = s.split(":");
  const [, , service, env, type, ...rest] = parts;
  const uuid = rest.join(":");
  if (!type || !uuid) return null;
  return { service, env, type, uuid, kind: ridKind(type) };
}

/** The bare entity_type token of a RID (the registry key), or null. */
export function ridType(s: string | null | undefined): string | null {
  return parseRid(s)?.type ?? null;
}

/** Ontology kind from an entity_type slot. */
export function ridKind(type: string): "object" | "link" | "action" {
  if (type.startsWith("link__")) return "link";
  if (type.startsWith("action__")) return "action";
  return "object";
}

/** Short, stable id tail for compact display when no code/name is available. */
export function ridTail(s: string, n = 8): string {
  const uuid = parseRid(s)?.uuid ?? s;
  return uuid.length > n ? uuid.slice(-n) : uuid;
}
