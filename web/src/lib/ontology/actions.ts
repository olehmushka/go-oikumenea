// Action-invocation helpers (D-ActionInvocation, review-2026-09 R-33) — the console counterpart of the
// backend action catalog. An action carries its parameter schema (R-29) AND its endpoint binding
// (method/path/pathParams, single-sourced from the IR), so the generic runner can build and POST the
// request without a hand-authored URL. This module is the pure logic (type classification, path
// substitution, body coercion); the React surface is components/ontology/ActionRunner.tsx.

import type { ActionType, ActionParam, ActionEndpoint } from "@/lib/api/types";

// Flat scalar type tokens the runner renders as a single input. Everything else (list<…>, set<…>,
// map<…>, and nested objects — whose token is the object's lowercased name) is a structured body
// handled by the recursive renderer (Phase 3).
const FLAT = new Set([
  "string",
  "rid",
  "uuid",
  "boolean",
  "integer",
  "double",
  "safelong",
  "datetime",
  "enum",
  "bearertoken",
  "any",
]);

/** How the runner renders a param: a single input (flat scalar), a repeatable list of scalar inputs
 * (list/set of a flat type), or a raw-JSON editor (nested object, list-of-object, map — the deep shapes,
 * whose 5 host actions all also have a richer bespoke form). */
export type FieldKind = "flat" | "list" | "json";

export function fieldKind(type: string): FieldKind {
  if (FLAT.has(type)) return "flat";
  const m = /^(?:list|set)<(.+)>$/.exec(type);
  if (m && FLAT.has(m[1])) return "list";
  return "json";
}

/** The element type of a list<…>/set<…> token (defaults to string). */
export function listItemType(type: string): string {
  const m = /^(?:list|set)<(.+)>$/.exec(type);
  return m ? m[1] : "string";
}

export function isFlatParam(p: ActionParam): boolean {
  return FLAT.has(p.type);
}

export function isFlatBody(a: ActionType): boolean {
  return (a.parameters ?? []).every(isFlatParam);
}

/** An action is invocable at all when it carries an endpoint binding (purge-cascade erasures and the
 * bulk import.* plane have none — they are catalogued but not runnable). */
export function isInvocable(a: ActionType): a is ActionType & { endpoint: ActionEndpoint } {
  return !!a.endpoint;
}

/** Object-level invocability: invocable, with at most one path param (the object's own RID). Every body
 * shape is renderable (flat, list, or JSON); sub-resource actions (≥2 path params) are launched from a
 * row on the bespoke pages (Phase 4). */
export function isObjectLevelInvocable(a: ActionType): boolean {
  return isInvocable(a) && (a.endpoint.pathParams?.length ?? 0) <= 1;
}

/** Substitute the endpoint's path params from a name→value map (values already trimmed). */
export function buildPath(e: ActionEndpoint, values: Record<string, string>): string {
  let path = e.path;
  for (const name of e.pathParams ?? []) {
    path = path.replace(`{${name}}`, encodeURIComponent(values[name] ?? ""));
  }
  return path;
}

/** Coerce a raw string input to the JSON value its Conjure type expects. */
export function coerceValue(type: string, raw: string): unknown {
  switch (type) {
    case "boolean":
      return raw === "true" || raw === "on";
    case "integer":
    case "safelong":
    case "double":
      return raw === "" ? undefined : Number(raw);
    case "datetime":
      // datetime-local yields "YYYY-MM-DDTHH:mm"; send an RFC-3339 instant.
      return raw === "" ? undefined : new Date(raw).toISOString();
    default:
      return raw;
  }
}

/** Build the JSON request body from the params + their form values. Flat scalars coerce by type; a list
 * param holds a string[] (empties dropped, each element coerced by the item type); a json param holds
 * raw text parsed with JSON.parse (which THROWS on malformed input — the caller catches and surfaces it).
 * Optional params left blank are omitted so the server sees an absent optional, not an empty value. */
export function buildBody(params: ActionParam[], values: Record<string, unknown>): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const p of params) {
    const kind = fieldKind(p.type);
    if (kind === "flat") {
      if (p.type === "boolean") {
        body[p.name] = values[p.name] === true || values[p.name] === "true";
        continue;
      }
      const raw = String(values[p.name] ?? "").trim();
      if (raw === "" && !p.required) continue;
      body[p.name] = coerceValue(p.type, raw);
    } else if (kind === "list") {
      const arr = ((values[p.name] as string[] | undefined) ?? []).map((s) => s.trim()).filter((s) => s !== "");
      if (arr.length === 0 && !p.required) continue;
      const item = listItemType(p.type);
      body[p.name] = arr.map((s) => coerceValue(item, s));
    } else {
      const raw = String(values[p.name] ?? "").trim();
      if (raw === "" && !p.required) continue;
      body[p.name] = JSON.parse(raw === "" ? "{}" : raw);
    }
  }
  return body;
}

/** The HTML input mode for a flat param type. */
export function inputKind(type: string): "checkbox" | "number" | "datetime-local" | "text" {
  switch (type) {
    case "boolean":
      return "checkbox";
    case "integer":
    case "safelong":
    case "double":
      return "number";
    case "datetime":
      return "datetime-local";
    default:
      return "text";
  }
}
