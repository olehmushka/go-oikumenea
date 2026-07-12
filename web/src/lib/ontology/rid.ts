// RID = the self-describing primary key every entity carries (D-ResourceIdentifiers). It is a native
// UUIDv8 whose packed bits encode app | service | kind | type-code | timestamp | random. The console
// decodes those bits here — service, ontology kind (object/link/action), and a registry token — so it
// can still route, label, and render any RID without a server lookup (the object workspace foundation).
//
// Byte layout (0-indexed): byte6 = version(4b)=8 << 4 | kind(4b); byte7 = app; byte8 = variant(2b) <<
// 6 | service(6b); byte9 = type low 8 bits; byte10 high nibble = type high 4 bits.

// SERVICE_NAMES + RID_TYPES are GENERATED from pkg/rid (the boot-asserted source of truth) by
// tools/gen-ontology-mirror — never hand-edit them here (R-28, review-2026-09). RID_TYPES carries,
// per `${service}/${kind}/${typeCode}` triple, the registry token (the key registry.ts is keyed by)
// and the bare type name (a fallback label for any RID that has no rich registry view).
import { SERVICE_NAMES, RID_TYPES } from "./generated-rid";

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
  const info = RID_TYPES[`${d.service}/${d.kind}/${d.typeCode}`];
  const token = info?.token ?? (kind === "action" ? "action" : "");
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

/** The bare, human-readable type name of a RID from the generated registry (e.g. "account" for a
 *  finance account RID), or null. A fallback label for any RID without a rich registry.ts view. */
export function ridTypeName(s: string | null | undefined): string | null {
  const d = decode(s ?? "");
  if (!d || d.version !== 8 || d.app !== APP) return null;
  return RID_TYPES[`${d.service}/${d.kind}/${d.typeCode}`]?.name ?? null;
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
