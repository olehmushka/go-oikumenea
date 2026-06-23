import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader } from "@/components/ui";
import { T } from "@/components/T";
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
        title={<T>Rank scheme</T>}
        description={<T>The system-wide rank scheme: system → category → type → rank (directory seniority only — rank is never authority). NATO STANAG-2116 grade codes give cross-system equivalence. Add, edit, delete, and import below.</T>}
      />
      {error ? <ErrorNotice error={error} /> : null}
      {scheme && scheme.systems.length === 0 && (
        <p className="mb-3 text-sm text-slate-500">
          <T>The rank scheme is empty — add a system below, or import a preset.</T>
        </p>
      )}
      {scheme ? (
        <RankSchemeManager scheme={scheme} grades={grades} />
      ) : !error ? (
        <EmptyState><T>Loading…</T></EmptyState>
      ) : null}
      <ImportRankScheme />
    </div>
  );
}
