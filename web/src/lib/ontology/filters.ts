// The URL kernel for the explorer's filter state (M56 ticket 4, D-ConsoleDashboards).
//
// List and dashboard are two renderings of ONE request state, and that state lives entirely in the
// URL. Everything that reads or writes that state goes through this file: the outbound API query,
// the pager, the list↔tree (and M57's list↔dashboard) toggle, the filter chips and "clear all".
// Before this existed the page hand-concatenated its query string in four places, which is why the
// ?view=tree toggle silently dropped every filter but `org`.
//
// Isomorphic and pure: imports only the registry, so the Server Component and the "use client"
// FilterBar share one implementation rather than two that agree by inspection.

import type { ObjectTypeDef } from "./registry";

/** Next's searchParams (a param may repeat) normalized to a URLSearchParams. */
export function toSearchParams(
  sp: Record<string, string | string[] | undefined>,
): URLSearchParams {
  const out = new URLSearchParams();
  for (const [k, v] of Object.entries(sp)) {
    if (v === undefined) continue;
    // A repeated param is a client mistake here (no filter is multi-valued yet); take the first so
    // ?status=a&status=b narrows deterministically instead of sending both.
    out.set(k, Array.isArray(v) ? (v[0] ?? "") : v);
  }
  return out;
}

/** Every contract arg name this type's filters may send — the whitelist for outbound queries. */
export function filterParams(def: ObjectTypeDef): string[] {
  return (def.filters ?? []).flatMap((f) => f.params);
}

/** The current value of each declared filter param, empty ones dropped. */
export function readFilters(def: ObjectTypeDef, sp: URLSearchParams): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of filterParams(def)) {
    const v = (sp.get(p) ?? "").trim();
    if (v) out[p] = v;
  }
  return out;
}

/** True when every `required` filter has a value — the gate for firing the list request at all
 *  (listUnits rejects an unscoped listing, so an unset `org` must not become a 400). */
export function requiredFiltersSatisfied(def: ObjectTypeDef, sp: URLSearchParams): boolean {
  return (def.filters ?? [])
    .filter((f) => f.required)
    .every((f) => f.params.every((p) => (sp.get(p) ?? "").trim() !== ""));
}

/** The free-text search term, only for a type whose list endpoint actually ships a search arg. The
 *  URL name is `q`; the wire name is def.list.searchParam. The two must not converge — `q` is not a
 *  filter param and must never be forwarded raw. */
export function readQuery(def: ObjectTypeDef, sp: URLSearchParams): string {
  return def.list?.searchParam ? (sp.get("q") ?? "").trim() : "";
}

/**
 * The outbound API query string: the type's default (`?pageSize=50`), then each whitelisted filter,
 * then the search arg, then the page cursor. Only declared params are forwarded — a junk param in
 * the URL survives navigation (link stability) but never reaches the backend.
 */
export function apiQuery(def: ObjectTypeDef, sp: URLSearchParams): string {
  const q = new URLSearchParams((def.list?.search ?? "").replace(/^\?/, ""));
  for (const [name, value] of Object.entries(readFilters(def, sp))) q.set(name, value);
  const term = readQuery(def, sp);
  if (term && def.list?.searchParam) q.set(def.list.searchParam, term);
  const pageToken = (sp.get("pageToken") ?? "").trim();
  if (pageToken) q.set("pageToken", pageToken);
  const s = q.toString();
  return s ? `?${s}` : "";
}

/**
 * The outbound STATS query (M57): exactly the list's filter args — the endpoint takes the same set —
 * minus paging, plus the `facets` CSV. Built from the same `readFilters`/`readQuery` whitelist as
 * `apiQuery`, because a dashboard that filtered differently from its list would make `totalCount`
 * disagree with the rows the operator can page to, which is the one property M57 promises.
 *
 * `overrides` applies the extra narrowing a multi-call chart needs (the pyramid's `sex` wings), and
 * an `undefined` there clears an inherited filter.
 *
 * `facets` MUST be non-empty: `facets=` is the wire form of "count only, no distributions", so
 * sending an accidental empty list draws an empty dashboard rather than the whole one.
 */
export function statsQuery(
  def: ObjectTypeDef,
  sp: URLSearchParams,
  facets: string,
  overrides: Record<string, string | undefined> = {},
): string {
  const q = new URLSearchParams();
  for (const [name, value] of Object.entries(readFilters(def, sp))) q.set(name, value);
  const term = readQuery(def, sp);
  if (term && def.list?.searchParam) q.set(def.list.searchParam, term);
  for (const [name, value] of Object.entries(overrides)) {
    if (value === undefined || value === "") q.delete(name);
    else q.set(name, value);
  }
  q.set("facets", facets);
  return `?${q.toString()}`;
}

/**
 * A URL for the same explorer with `patch` applied — `undefined` deletes a param.
 *
 * `pageToken` is ALWAYS dropped unless the patch sets it explicitly: a keyset cursor was minted
 * under one filter set and means nothing under another, so "changing a filter returns to page 1" is
 * a property of the builder rather than a discipline at each call site.
 */
export function exploreHref(
  type: string,
  sp: URLSearchParams,
  patch: Record<string, string | undefined> = {},
): string {
  const next = new URLSearchParams(sp);
  for (const [k, v] of Object.entries(patch)) {
    if (v === undefined || v === "") next.delete(k);
    else next.set(k, v);
  }
  if (!("pageToken" in patch)) next.delete("pageToken");
  const s = next.toString();
  return s ? `/explore/${type}?${s}` : `/explore/${type}`;
}

/** Everything except the cursor, for <Pager extraQuery> (which appends its own pageToken). */
export function exploreExtraQuery(sp: URLSearchParams): string {
  const next = new URLSearchParams(sp);
  next.delete("pageToken");
  return next.toString();
}

/**
 * True when a STRUCTURAL filter is applied — a required one (org) does not count, since it is the
 * precondition for listing at all rather than a narrowing. Free-text search is deliberately not
 * counted either: it has its own empty-state wording ("No matches." vs "No matches for these
 * filters."), and conflating them would misdescribe an empty search on a search-only catalog.
 */
export function hasActiveFilters(def: ObjectTypeDef, sp: URLSearchParams): boolean {
  const active = readFilters(def, sp);
  return (def.filters ?? [])
    .filter((f) => !f.required)
    .flatMap((f) => f.params)
    .some((p) => active[p] !== undefined);
}
