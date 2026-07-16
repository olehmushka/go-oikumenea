import { oikumenea } from "@/lib/api/server";
import { Card } from "@/components/ui";
import { T } from "@/components/T";
import { ActionRunner } from "./ActionRunner";
import type { ActionType } from "@/lib/api/types";

/** Collapsed "advanced" panel of a type's catalogued actions, runnable inline (D-ActionInvocation,
 * review-2026-09 R-33). Mounted on the bespoke person/unit pages — which /o/[rid] redirects to — so they
 * gain the same generic action surface as the universal object view without disturbing their curated,
 * bespoke managers (object-level actions run inline here; sub-resource update/delete stay on the managers
 * above, which carry the child id). A server component: it fetches the static catalog and filters by type. */
export async function ObjectActionsCard({
  targetType,
  rid,
  className,
}: {
  targetType: string;
  rid: string;
  className?: string;
}) {
  let actions: ActionType[] = [];
  try {
    const cat = await oikumenea().then((ok) => ok.audit.listActionTypes());
    actions = cat.filter((a) => a.targetType === targetType);
  } catch {
    return null;
  }
  if (actions.length === 0) return null;

  return (
    <Card className={className}>
      <details>
        <summary className="cursor-pointer text-sm font-semibold text-slate-900">
          <T>Actions (advanced)</T>
        </summary>
        <p className="mb-2 mt-1 text-xs text-slate-500">
          <T>
            Every catalogued action for this type (D-ActionTypes, R-29), each bound to its endpoint
            (D-ActionInvocation, R-33). Object-level actions run inline; sub-resource update/delete run
            from the managers above, which carry the row id.
          </T>
        </p>
        <ActionRunner actions={actions} rid={rid} />
      </details>
    </Card>
  );
}
