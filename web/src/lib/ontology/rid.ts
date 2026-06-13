// RID = the self-describing primary key every entity carries (D-ResourceIdentifiers). It is a native
// UUIDv8 whose packed bits encode app | service | kind | type-code | timestamp | random. The console
// decodes those bits here — service, ontology kind (object/link/action), and a registry token — so it
// can still route, label, and render any RID without a server lookup (the object workspace foundation).
//
// Byte layout (0-indexed): byte6 = version(4b)=8 << 4 | kind(4b); byte7 = app; byte8 = variant(2b) <<
// 6 | service(6b); byte9 = type low 8 bits; byte10 high nibble = type high 4 bits.

export interface ParsedRid {
  service: string;
  /** retained for backward compatibility; the environment is no longer encoded in the id */
  env: string;
  /** registry token, e.g. "person", "link__member_of", "action" — matches registry.ts keys */
  type: string;
  uuid: string;
  /** ontology kind decoded from the packed kind nibble */
  kind: "object" | "link" | "action";
}

const APP = 1;

const SERVICE_NAMES: Record<number, string> = {
  1: "platform", 2: "i18n", 3: "audit", 4: "tenant", 5: "rank", 6: "person",
  7: "membership", 8: "authz", 9: "account", 10: "document", 11: "order",
};

// Token by `${service}/${kind}/${typeCode}` — kind 1=object, 2=link, 3=action. Mirrors the SQL
// platform_rid_types / pkg/rid registry, but emits the registry.ts key tokens (link__*, rank_system).
const TYPE_TOKENS: Record<string, string> = {
  "2/1/1": "translation",
  "4/1/1": "unit", "4/1/2": "graph", "4/1/3": "unit_lifecycle_event", "4/2/1": "link__parent_of",
  "5/1/1": "rank_system", "5/1/2": "rank_category", "5/1/3": "rank_type", "5/1/4": "rank",
  "6/1/1": "person", "6/1/2": "name_variant", "6/1/3": "citizenship", "6/1/4": "residence",
  "6/1/5": "email", "6/1/6": "phone", "6/1/7": "call_sign", "6/1/8": "messenger_link",
  "6/1/9": "social_account", "6/1/10": "social_handle",
  "6/2/1": "link__holds_rank", "6/2/2": "link__partnered_with", "6/2/3": "link__kin_parent_of",
  "6/2/4": "link__guardian_of", "6/2/5": "link__sponsor_of", "6/2/6": "link__next_of_kin",
  "6/2/7": "link__associated_with",
  "7/1/1": "position", "7/2/1": "link__member_of",
  "8/1/1": "role", "8/2/1": "link__has_role", "8/2/2": "link__instance_admin",
  "9/1/1": "account", "9/1/2": "external_identity",
  "10/1/1": "document_type", "10/1/2": "document", "10/1/3": "personal_code",
  "11/1/1": "order_type", "11/1/2": "order", "11/1/3": "order_item",
};

interface Decoded {
  app: number;
  service: number;
  kind: number;
  version: number;
  typeCode: number;
}

function decode(s: string): Decoded | null {
  const hex = s.replace(/-/g, "");
  if (!/^[0-9a-fA-F]{32}$/.test(hex)) return null;
  const b = (i: number) => parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  return {
    app: b(7),
    service: b(8) & 0x3f,
    kind: b(6) & 0x0f,
    version: b(6) >> 4,
    typeCode: b(9) | ((b(10) >> 4) << 8),
  };
}

const KINDS: Record<number, "object" | "link" | "action"> = { 1: "object", 2: "link", 3: "action" };

/** True for a string shaped like an oikumenea RID: a uuid carrying our app code and UUIDv8 version. */
export function isRid(s: string | null | undefined): s is string {
  if (typeof s !== "string") return false;
  const d = decode(s);
  return d !== null && d.version === 8 && d.app === APP;
}

/** Parse a RID into its decoded parts, or null if it isn't one. Never throws. */
export function parseRid(s: string | null | undefined): ParsedRid | null {
  if (typeof s !== "string") return null;
  const d = decode(s);
  if (!d || d.version !== 8 || d.app !== APP) return null;
  const kind = KINDS[d.kind] ?? "object";
  const token = TYPE_TOKENS[`${d.service}/${d.kind}/${d.typeCode}`] ?? (kind === "action" ? "action" : "");
  return {
    service: SERVICE_NAMES[d.service] ?? `s${d.service}`,
    env: "",
    type: token,
    uuid: s.toLowerCase(),
    kind,
  };
}

/** The registry token of a RID (the registry key), or null. */
export function ridType(s: string | null | undefined): string | null {
  return parseRid(s)?.type || null;
}

/** Ontology kind of a RID. (Kept compatible: also accepts a bare token string.) */
export function ridKind(s: string): "object" | "link" | "action" {
  const parsed = parseRid(s);
  if (parsed) return parsed.kind;
  // Backward-compatible fallback for callers passing a registry token rather than a RID.
  if (s.startsWith("link__")) return "link";
  if (s.startsWith("action__")) return "action";
  return "object";
}

/** Short, stable id tail for compact display when no code/name is available. */
export function ridTail(s: string, n = 8): string {
  const uuid = parseRid(s)?.uuid ?? s;
  return uuid.length > n ? uuid.slice(-n) : uuid;
}
