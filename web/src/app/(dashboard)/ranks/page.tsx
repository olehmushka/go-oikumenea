import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader } from "@/components/ui";
import { ImportRankScheme, RankSchemeManager } from "./RankSchemeManager";
import type { RankGrade, RankScheme } from "@/lib/api/types";

export default async function RanksPage() {
  let scheme: RankScheme | null = null;
  let grades: RankGrade[] = [];
  let error: unknown = null;
  try {
    [scheme, grades] = await Promise.all([
      apiGet<RankScheme>("/rank/v1/rank-scheme"),
      apiGet<RankGrade[]>("/rank/v1/rank-grades").catch(() => []),
    ]);
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title="Rank scheme"
        description="The system-wide rank scheme: system → category → type → rank (directory seniority only — rank is never authority). NATO STANAG-2116 grade codes give cross-system equivalence. Add, edit, delete, and import below."
      />
      {error ? <ErrorNotice error={error} /> : null}
      {scheme && scheme.systems.length === 0 && (
        <p className="mb-3 text-sm text-slate-500">
          The rank scheme is empty — add a system below, or import a preset.
        </p>
      )}
      {scheme ? (
        <RankSchemeManager scheme={scheme} grades={grades} />
      ) : !error ? (
        <EmptyState>Loading…</EmptyState>
      ) : null}
      <ImportRankScheme />
    </div>
  );
}
