import "server-only";
import { cache } from "react";
import { oikumenea } from "@/lib/api/server";
import { errorInfo } from "@/lib/api/errors";

/**
 * Best-effort UI gating: "may the signed-in user do <action>?"
 *
 * THIS IS NOT AN ENFORCEMENT POINT. It exists so instance-admin-only surfaces (the service-principal
 * console, M52) don't render a page of buttons that every non-admin sees 403 on. The server remains the
 * only authority — every endpoint re-decides through the PDP regardless of what the UI chose to draw,
 * and D-HeadlessTopology turns on exactly that (facades gain no authority). Never use `can()` to
 * *permit* anything; use it only to hide what would fail.
 *
 * How it decides, and why a thrown error means `false`:
 *
 * `Whoami` carries only the resolved subject id — no permission list — so there is nothing to read
 * locally and the question must go to the PDP. But `POST /authorize` is itself gated on
 * `assignment.read` with **no self-exemption** (OQ-5, api/authorization.conjure.yml): a user without it
 * is denied *the question*, not just the answer. That is the desired outcome anyway — if you cannot ask
 * whether you may manage service principals, you are certainly not an instance admin — so any failure
 * (denied, unreachable, malformed) collapses to `false`. The cost of a wrong `false` is a hidden menu
 * entry; the cost of a wrong `true` is a button that 403s, which is the status quo everywhere else in
 * the console.
 *
 * Longer term the clean answer is for the contract to expose the caller's own effective permissions
 * (an extension of `Whoami`), which would make this a single unprivileged read with no self-referential
 * gating. That is a contract change and deliberately out of M52's scope.
 */

/** Per-request memo (React `cache`): several gated surfaces on one page share a single whoami call. */
const subjectPersonId = cache(async (): Promise<string | null> => {
  try {
    const ok = await oikumenea();
    const me = await ok.identityFederation.whoami();
    return me.personId || null;
  } catch {
    return null;
  }
});

/**
 * `unitId` is omitted for instance-scope actions (`service-principal.read`, `role.create`, …) and
 * supplied for unit-scoped ones, matching AuthorizeRequest.
 */
export const can = cache(async (action: string, unitId?: string): Promise<boolean> => {
  const personId = await subjectPersonId();
  if (!personId) return false;

  try {
    const ok = await oikumenea();
    const decision = await ok.authorization.authorize({
      subjectPersonId: personId,
      action,
      unitId: unitId ?? null,
      explain: false,
    });
    return decision.allow === true;
  } catch (e) {
    // 403 = the caller lacks assignment.read, i.e. is no instance admin (see above). Anything else is
    // a transport/contract fault; both mean "don't offer the surface".
    const { status } = errorInfo(e);
    if (status !== undefined && status !== 403 && status !== 401) {
      console.warn(`can(${action}): unexpected failure, treating as denied`, status);
    }
    return false;
  }
});
