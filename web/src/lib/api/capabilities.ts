import "server-only";
import { cache } from "react";
import { oikumenea } from "@/lib/api/server";
import { type Capabilities, NO_CAPS, holds } from "@/lib/ontology/caps";

/**
 * The signed-in caller's own effective permissions, fetched once per request (React `cache`) from
 * GET /me/capabilities (D-SelfCapabilities). Unlike the old per-code `POST /authorize` probes, this
 * is a single UNGATED read: a user without `assignment.read` still learns their own codes, so module
 * hiding works for ordinary users, not just admins.
 *
 * NOT AN ENFORCEMENT POINT — the API re-decides on every call regardless of what the UI drew. Any
 * failure (unreachable, malformed) collapses to NO_CAPS: fail closed on a *display* decision (a
 * hidden menu entry, never a silently-permitted action).
 */
export const capabilities = cache(async (): Promise<Capabilities> => {
  try {
    const ok = await oikumenea();
    const caps = await ok.authorization.myCapabilities();
    return {
      permissions: caps.permissions ?? [],
      isInstanceAdmin: caps.isInstanceAdmin === true,
    };
  } catch {
    return NO_CAPS;
  }
});

/** Server-side convenience: does the signed-in caller hold `code`? Backs the reworked `can()`. */
export async function holdsCode(code: string): Promise<boolean> {
  return holds(await capabilities(), code);
}
