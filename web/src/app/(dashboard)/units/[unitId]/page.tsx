import Link from "next/link";
import { apiGet } from "@/lib/api/server";
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
import { EdgeManager } from "@/components/EdgeManager";
import { CreatePosition, FillPosition, PositionAdmin } from "@/components/PositionForms";
import { UnitAdmin } from "@/components/UnitForms";
import type { Position, Unit, UnitRefList } from "@/lib/api/types";

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
  let error: unknown = null;
  try {
    unit = await apiGet<Unit>(`/tenant/v1/units/${unitId}`);
    [ancestors, descendants, positions] = await Promise.all([
      apiGet<UnitRefList>(`/tenant/v1/units/${unitId}/ancestors`).catch(() => null),
      apiGet<UnitRefList>(`/tenant/v1/units/${unitId}/descendants`).catch(() => null),
      apiGet<{ positions: Position[] }>(
        `/membership/v1/units/${unitId}/positions`,
      ).catch(() => null),
    ]);
  } catch (e) {
    error = e;
  }

  if (error) {
    return (
      <div>
        <PageHeader title="Unit" />
        <ErrorNotice error={error} />
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title={unit?.code ?? unitId}
        description="Unit detail, graph neighbourhood, and positions."
        action={
          <Link href="/units" className="btn-ghost">
            ← All units
          </Link>
        }
      />

      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Details</h2>
          <dl className="mt-3 space-y-2 text-sm">
            <Row label="Name" value={<Localized map={unit?.name} />} />
            <Row label="Code" value={<Mono>{unit?.code}</Mono>} />
            <Row label="Kind" value={unit?.unitKind ?? "—"} />
            <Row label="Level" value={unit?.level ?? "—"} />
            <Row
              label="Visibility"
              value={
                <Pill tone={unit?.visibility === "SHADOW" ? "amber" : "green"}>
                  {unit?.visibility ?? "—"}
                </Pill>
              }
            />
            <Row
              label="State"
              value={
                <Pill tone={unit?.state === "ACTIVE" ? "green" : "slate"}>
                  {unit?.state ?? "—"}
                </Pill>
              }
            />
            <Row label="ID" value={<Mono>{unit?.id}</Mono>} />
          </dl>
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Ancestors</h2>
          <UnitRefs refs={ancestors?.units} empty="No parents (a root unit)." />
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Descendants</h2>
          <UnitRefs refs={descendants?.units} empty="No descendants." />
        </Card>
      </div>

      <Card className="mt-4">
        <h2 className="text-sm font-semibold text-slate-900">Manage unit</h2>
        <p className="mb-3 mt-1 text-xs text-slate-500">
          Edit details, or move it through its lifecycle (archive is the equivalent of delete).
        </p>
        {unit ? <UnitAdmin unit={unit} /> : null}
      </Card>

      <Card className="mt-4">
        <h2 className="text-sm font-semibold text-slate-900">Edges</h2>
        <p className="mb-3 mt-1 text-xs text-slate-500">
          Nest this unit under a parent (creates a child relationship in the chosen graph).
        </p>
        <EdgeManager unitId={unitId} />
      </Card>

      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900">Positions</h2>
      {positions && positions.positions?.length > 0 ? (
        <Table
          head={
            <>
              <th className="th">Code</th>
              <th className="th">Title</th>
              <th className="th">Status</th>
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
                <Pill tone={p.status === "FILLED" ? "green" : "slate"}>
                  {p.status ?? "—"}
                </Pill>
              </td>
              <td className="td">
                <div className="relative flex items-center justify-end gap-3">
                  {p.status !== "abolished" ? <FillPosition positionId={p.id} /> : null}
                  <PositionAdmin position={p} />
                </div>
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState>No positions defined for this unit.</EmptyState>
      )}
      <div className="mt-4 max-w-xl">
        <CreatePosition unitId={unitId} />
      </div>
    </div>
  );
}

function UnitRefs({
  refs,
  empty,
}: {
  refs?: { id: string; code: string; name?: Record<string, string> }[];
  empty: string;
}) {
  if (!refs || refs.length === 0)
    return <p className="mt-3 text-sm text-slate-400">{empty}</p>;
  return (
    <ul className="mt-3 space-y-1 text-sm">
      {refs.map((r) => (
        <li key={r.id}>
          <Link href={`/units/${r.id}`} className="text-indigo-600 hover:underline">
            <Localized map={r.name} fallback={r.code} />
          </Link>
        </li>
      ))}
    </ul>
  );
}

function Row({ label, value }: { label: string; value?: React.ReactNode }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-slate-500">{label}</dt>
      <dd className="text-right text-slate-800">{value ?? "—"}</dd>
    </div>
  );
}
