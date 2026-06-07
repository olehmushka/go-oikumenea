import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader } from "@/components/ui";
import { RankSchemeManager } from "./RankSchemeManager";
import type { RankScheme } from "@/lib/api/types";

export default async function RanksPage() {
  let scheme: RankScheme | null = null;
  let error: unknown = null;
  try {
    scheme = await apiGet<RankScheme>("/rank/v1/rank-scheme");
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title="Rank scheme"
        description="The single system-wide rank scheme: category → type → rank (directory seniority only — rank is never authority). Add, edit, and delete nodes below."
      />
      {error ? <ErrorNotice error={error} /> : null}
      {scheme && scheme.categories.length === 0 && (
        <p className="mb-3 text-sm text-slate-500">The rank scheme is empty — add a category below.</p>
      )}
      {scheme ? <RankSchemeManager scheme={scheme} /> : !error ? <EmptyState>Loading…</EmptyState> : null}
    </div>
  );
}
