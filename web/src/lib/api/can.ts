import "server-only";
import { cache } from "react";
import { oikumenea } from "@/lib/api/server";
import { errorInfo } from "@/lib/api/errors";
import { holdsCode } from "@/lib/api/capabilities";

/**
 * Best-effort UI gating: "may the signed-in user do <action>?"
 *
 * THIS IS NOT AN ENFORCEMENT POINT. It exists so surfaces the caller can't use don't render a page of
 * buttons that 403. The server remains the only authority — every endpoint re-decides through the PDP
 * regardless of what the UI drew (D-HeadlessTopology). Never use `can()` to *permit* anything; use it
 * only to hide what would fail.
 *
 * Two paths:
 *
 * - **Instance-scope / "holds anywhere" checks** (no `unitId`): answered from the caller's own
 *   effective permissions via GET /me/capabilities (D-SelfCapabilities) — a single unprivileged read.
 *   This replaced the old per-code `POST /authorize` probe, which needed `assignment.read` and so
 *   failed closed for every non-admin. This is what all current call sites use.
 *
 * - **Unit-scoped checks** (`unitId` given): a specific `(action, unit)` question the flat capability
 *   set can't answer, so it still goes to `POST /authorize`. That endpoint is gated on
 *   `assignment.read` with no self-exemption, so this path only yields `true` for callers holding it;
 *   any failure (denied, unreachable) collapses to `false`. No console surface uses this path today —
 *   it is kept for completeness.
 */
export const can = cache(async (action: string, unitId?: string): Promise<boolean> => {
  if (unitId === undefined) {
    // Module / instance-scope gating — the common case.
    return holdsCode(action);
  }
  return canAtUnit(action, unitId);
});

/** Per-request memo of the caller's subject id, for the unit-scoped PDP path only. */
const subjectPersonId = cache(async (): Promise<string | null> => {
  try {
    const ok = await oikumenea();
    const me = await ok.identityFederation.whoami();
    return me.personId || null;
  } catch {
    return null;
  }
});

async function canAtUnit(action: string, unitId: string): Promise<boolean> {
  const personId = await subjectPersonId();
  if (!personId) return false;
  try {
    const ok = await oikumenea();
    const decision = await ok.authorization.authorize({
      subjectPersonId: personId,
      action,
      unitId,
      explain: false,
    });
    return decision.allow === true;
  } catch (e) {
    const { status } = errorInfo(e);
    if (status !== undefined && status !== 403 && status !== 401) {
      console.warn(`can(${action}, ${unitId}): unexpected failure, treating as denied`, status);
    }
    return false;
  }
}
