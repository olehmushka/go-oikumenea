import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, PageHeader, Pill, Table } from "@/components/ui";
import { LookupForm } from "@/components/LookupForm";
import { AddMembership, EndMembershipButton } from "@/components/MembershipForms";
import type { Membership, Position } from "@/lib/api/types";

export default async function MembershipsPage({
  searchParams,
}: {
  searchParams: Promise<{ unitId?: string; personId?: string }>;
}) {
  const { unitId, personId } = await searchParams;
  let rows: Membership[] | null = null;
  let positions: Position[] = [];
  let error: unknown = null;
  const target = unitId
    ? `/membership/v1/units/${unitId}/members`
    : personId
      ? `/membership/v1/persons/${personId}/memberships`
      : null;
  if (target) {
    try {
      const r = await apiGet<{ memberships: Membership[] }>(target);
      rows = r.memberships ?? [];
    } catch (e) {
      error = e;
    }
  }
  // When viewing a unit, load its positions so the Add-membership form can assign to a billet.
  if (unitId) {
    positions = await apiGet<{ positions: Position[] }>(
      `/membership/v1/units/${unitId}/positions`,
    )
      .then((r) => r.positions ?? [])
      .catch(() => []);
  }

  return (
    <div>
      <PageHeader
        title="Memberships"
        description="person ↔ unit belonging and the positions they hold. Look up by unit or by person."
      />

      <div className="mb-5 grid gap-4 sm:grid-cols-2">
        <LookupForm
          basePath="/memberships"
          param="unitId"
          label="By unit"
          kind="unit"
          current={unitId}
        />
        <LookupForm
          basePath="/memberships"
          param="personId"
          label="By person"
          kind="person"
          current={personId}
        />
      </div>

      {error ? <ErrorNotice error={error} /> : null}
      {rows && rows.length === 0 && <EmptyState>No memberships found.</EmptyState>}
      {rows && rows.length > 0 && (
        <Table
          head={
            <>
              <th className="th">Person</th>
              <th className="th">Unit</th>
              <th className="th">Position</th>
              <th className="th">Status</th>
              <th className="th">From</th>
              <th className="th"></th>
            </>
          }
        >
          {rows.map((m) => (
            <tr key={m.id}>
              <td className="td">
                <Link
                  href={`/persons/${m.personId}`}
                  className="text-indigo-600 hover:underline"
                >
                  <Mono>{m.personId.slice(-8)}</Mono>
                </Link>
              </td>
              <td className="td">
                <Link
                  href={`/units/${m.unitId}`}
                  className="text-indigo-600 hover:underline"
                >
                  <Mono>{m.unitId.slice(-8)}</Mono>
                </Link>
              </td>
              <td className="td">
                {m.positionId ? <Mono>{m.positionId.slice(-8)}</Mono> : "—"}
              </td>
              <td className="td">
                <Pill tone={m.status === "ACTIVE" ? "green" : "slate"}>
                  {m.status ?? "—"}
                </Pill>
              </td>
              <td className="td">{m.effectiveFrom ?? "—"}</td>
              <td className="td text-right">
                {(m.status ?? "").toUpperCase() !== "ENDED" ? (
                  <EndMembershipButton membershipId={m.id} />
                ) : null}
              </td>
            </tr>
          ))}
        </Table>
      )}

      {unitId && (
        <div className="mt-6 max-w-xl">
          <AddMembership unitId={unitId} positions={positions} />
        </div>
      )}
    </div>
  );
}
