import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import {
  Card,
  EmptyState,
  ErrorNotice,
  Mono,
  PageHeader,
  Pill,
  Table,
} from "@/components/ui";
import { Localized } from "@/components/Localized";
import { T } from "@/components/T";
import { EdgeManager } from "@/components/EdgeManager";
import { CreatePosition, FillPosition, PersonLink, PositionAdmin } from "@/components/PositionForms";
import { UnitAdmin } from "@/components/UnitForms";
import { UnitLanguageManager } from "@/components/UnitLanguageForms";
import { HistoryPanel } from "@/components/ontology/HistoryPanel";
import { ObjectActionsCard } from "@/components/ontology/ObjectActionsCard";
import type { Position, Unit, UnitCodeEventList, UnitRefList } from "@/lib/api/types";

export default async function UnitDetailPage({
  params,
}: {
  params: Promise<{ unitId: string }>;
}) {
  const { unitId } = await params;
  let unit: Unit | null = null;
  let ancestors: UnitRefList | null = null;
  let descendants: UnitRefList | null = null;
  let positions: { positions: Position[] } | null = null;
  let codeEvents: UnitCodeEventList | null = null;
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    unit = await ok.tenant.getUnit(unitId);
    [ancestors, descendants, positions, codeEvents] = await Promise.all([
      ok.tenant.unitAncestors(unitId).catch(() => null),
      ok.tenant.unitDescendants(unitId).catch(() => null),
      ok.membership.listPositions(unitId).catch(() => null),
      ok.tenant.listUnitCodeEvents(unitId).catch(() => null),
    ]);
  } catch (e) {
    error = e;
  }

  if (error) {
    return (
      <div>
        <PageHeader title={<T>Unit</T>} />
        <ErrorNotice error={error} />
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title={unit?.code ?? unitId}
        description={<T>Unit detail, graph neighbourhood, and positions.</T>}
        action={
          <Link href="/units" className="btn-ghost">
            ← <T>All units</T>
          </Link>
        }
      />

      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Details</T></h2>
          <dl className="mt-3 space-y-2 text-sm">
            <Row label={<T>Name</T>} value={<Localized map={unit?.name} />} />
            <Row
              label={<T>Code</T>}
              value={unit?.code ? <Mono>{unit.code}</Mono> : <span className="text-slate-400"><T>none (sub-unit)</T></span>}
            />
            <Row label={<T>Kind</T>} value={unit?.kindId ? <Mono>{unit.kindId}</Mono> : "—"} />
            <Row label={<T>Level</T>} value={unit?.level ?? "—"} />
            <Row
              label={<T>Visibility</T>}
              value={
                <Pill tone={unit?.visibility === "SHADOW" ? "amber" : "green"}>
                  {unit?.visibility ?? "—"}
                </Pill>
              }
            />
            <Row
              label={<T>State</T>}
              value={
                <Pill tone={unit?.state === "ACTIVE" ? "green" : "slate"}>
                  {unit?.state ?? "—"}
                </Pill>
              }
            />
            <Row label={<T>ID</T>} value={<Mono>{unit?.id}</Mono>} />
          </dl>
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Ancestors</T></h2>
          <UnitRefs refs={ancestors?.units} empty="No parents (a root unit)." />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Descendants</T></h2>
          <UnitRefs refs={descendants?.units} empty="No descendants." />
        </Card>
      </div>

      <Card className="mt-4">
        <h2 className="text-sm font-semibold text-slate-900"><T>Manage unit</T></h2>
        <p className="mb-3 mt-1 text-xs text-slate-500">
          <T>Edit details, or move it through its lifecycle (archive is the equivalent of delete).</T>
        </p>
        {unit ? <UnitAdmin unit={unit} /> : null}
      </Card>

      {codeEvents && codeEvents.events.length > 0 ? (
        <Card className="mt-4">
          <h2 className="text-sm font-semibold text-slate-900"><T>Code history</T></h2>
          <p className="mb-3 mt-1 text-xs text-slate-500">
            <T>Every set / correct / clear of this unit's code (newest first; D-UnitCodeLifecycle).</T>
          </p>
          <Table
            head={
              <>
                <th className="th"><T>When</T></th>
                <th className="th"><T>From</T></th>
                <th className="th"><T>To</T></th>
                <th className="th"><T>Reason</T></th>
              </>
            }
          >
            {codeEvents.events.map((e) => (
              <tr key={e.id}>
                <td className="td">{new Date(e.createdAt).toLocaleString()}</td>
                <td className="td">{e.oldCode ? <Mono>{e.oldCode}</Mono> : <span className="text-slate-400">—</span>}</td>
                <td className="td">{e.newCode ? <Mono>{e.newCode}</Mono> : <span className="text-slate-400">—</span>}</td>
                <td className="td text-slate-600">{e.reason ?? "—"}</td>
              </tr>
            ))}
          </Table>
        </Card>
      ) : null}

      <Card className="mt-4">
        <h2 className="text-sm font-semibold text-slate-900"><T>Edges</T></h2>
        <p className="mb-3 mt-1 text-xs text-slate-500">
          <T>Nest this unit under a parent (creates a child relationship in the chosen graph).</T>
        </p>
        <EdgeManager unitId={unitId} />
      </Card>

      <Card className="mt-4">
        <h2 className="text-sm font-semibold text-slate-900"><T>Languages</T></h2>
        <p className="mb-2 mt-1 text-xs text-slate-500">
          <T>The unit's official / working languages (D-Languages).</T>
        </p>
        <UnitLanguageManager unitId={unitId} />
      </Card>

      <Card className="mt-4">
        <h2 className="text-sm font-semibold text-slate-900"><T>History</T></h2>
        <p className="mb-3 mt-1 text-xs text-slate-500">
          <T>Every recorded change to this unit, newest first (audit ledger; D-Temporal, R-31).</T>
        </p>
        <HistoryPanel rid={unitId} />
      </Card>

      <ObjectActionsCard targetType="unit" rid={unitId} className="mt-4" />

      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900"><T>Positions</T></h2>
      {positions && positions.positions?.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Code</T></th>
              <th className="th"><T>Title</T></th>
              <th className="th"><T>Holder</T></th>
              <th className="th"><T>Status</T></th>
              <th className="th"></th>
            </>
          }
        >
          {positions.positions.map((p) => (
            <tr key={p.id}>
              <td className="td">
                <Mono>{p.code}</Mono>
              </td>
              <td className="td">
                <Localized map={p.title} />
              </td>
              <td className="td">
                {p.holder?.personId ? (
                  <PersonLink personId={p.holder.personId} />
                ) : (
                  <span className="text-slate-400"><T>vacant</T></span>
                )}
              </td>
              <td className="td">
                <Pill tone={p.holder ? "green" : p.status === "abolished" ? "slate" : "amber"}>
                  {p.status === "abolished" ? <T>abolished</T> : p.holder ? <T>filled</T> : <T>vacant</T>}
                </Pill>
              </td>
              <td className="td">
                <div className="relative flex items-center justify-end gap-3">
                  {p.status !== "abolished" && !p.holder ? <FillPosition positionId={p.id} /> : null}
                  <PositionAdmin position={p} />
                </div>
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState><T>No positions defined for this unit.</T></EmptyState>
      )}
      <div className="mt-4 max-w-xl">
        <CreatePosition unitId={unitId} />
      </div>
    </div>
  );
}

function UnitRefs({ refs, empty }: { refs?: UnitRefList["units"]; empty: string }) {
  if (!refs || refs.length === 0)
    return <p className="mt-3 text-sm text-slate-400"><T>{empty}</T></p>;
  return (
    <ul className="mt-3 space-y-1 text-sm">
      {refs.map((r) => (
        <li key={r.id}>
          <Link href={`/units/${r.id}`} className="text-indigo-600 hover:underline">
            <Localized map={r.name} fallback={r.code ?? undefined} />
          </Link>
        </li>
      ))}
    </ul>
  );
}

function Row({ label, value }: { label: React.ReactNode; value?: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-slate-500">{label}</dt>
      <dd className="text-right text-slate-800">{value ?? "—"}</dd>
    </div>
  );
}
