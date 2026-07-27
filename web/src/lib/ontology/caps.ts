// Client-safe capability primitives (no "server-only"): the shape returned by the self-capabilities
// endpoint (D-SelfCapabilities) plus the pure predicate the nav/palette/ontology surfaces use to
// decide whether to DRAW an entry. This is cosmetic gating only — the server re-decides every request
// regardless of what the UI chose to render (see lib/api/can.ts, lib/api/capabilities.ts).

export type Capabilities = {
  /** the caller's own effective permission codes (empty when isInstanceAdmin) */
  permissions: string[];
  /** true ⇒ holds everything; the UI treats every code as granted */
  isInstanceAdmin: boolean;
};

/** Empty capabilities — used as a fail-closed default when the endpoint can't be reached. */
export const NO_CAPS: Capabilities = { permissions: [], isInstanceAdmin: false };

/**
 * Does the caller hold `code` (anywhere)? An instance admin holds everything. A `null`/`undefined`
 * code means "no gating code known for this surface" → always shown, matching the console's
 * historical open-by-default behaviour for entries without a `requires`.
 */
export function holds(caps: Capabilities, code: string | null | undefined): boolean {
  if (!code) return true;
  return caps.isInstanceAdmin || caps.permissions.includes(code);
}
