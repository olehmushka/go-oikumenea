import { apiGet } from "@/lib/api/server";
import {
  Card,
  EmptyState,
  ErrorNotice,
  Mono,
  Pill,
  PageHeader,
  Table,
} from "@/components/ui";
import { Localized } from "@/components/Localized";
import { DeleteButton } from "@/components/DeleteButton";
import { LookupForm } from "@/components/LookupForm";
import {
  AssignmentGrant,
  EditRole,
  InstanceAdminGrant,
  RoleCreate,
} from "./RoleForms";
import type { AssignmentPage, RolePage } from "@/lib/api/types";

export default async function RolesPage({
  searchParams,
}: {
  searchParams: Promise<{ subjectPersonId?: string; targetUnitId?: string }>;
}) {
  const { subjectPersonId, targetUnitId } = await searchParams;
  let roles: RolePage | null = null;
  let assignments: AssignmentPage | null = null;
  let error: unknown = null;
  try {
    const aq = new URLSearchParams({ pageSize: "50" });
    if (subjectPersonId) aq.set("subjectPersonId", subjectPersonId);
    if (targetUnitId) aq.set("targetUnitId", targetUnitId);
    [roles, assignments] = await Promise.all([
      apiGet<RolePage>("/authorization/v1/roles", "?pageSize=100"),
      apiGet<AssignmentPage>("/authorization/v1/assignments", `?${aq}`),
    ]);
  } catch (e) {
    error = e;
  }

  const roleById = new Map((roles?.roles ?? []).map((r) => [r.id, r]));

  return (
    <div>
      <PageHeader
        title="Roles &amp; access"
        description="RBAC: code-defined permissions packaged into roles, then granted as scoped assignments. Authority comes only from assignments — never rank or position."
      />
      {error ? <ErrorNotice error={error} /> : null}

      {/* Roles */}
      <h2 className="mb-3 text-sm font-semibold text-slate-900">Roles</h2>
      {roles && roles.roles.length > 0 ? (
        <Table
          head={
            <>
              <th className="th">Code</th>
              <th className="th">Name</th>
              <th className="th">Permissions</th>
              <th className="th">Base</th>
              <th className="th"></th>
            </>
          }
        >
          {roles.roles.map((r) => (
            <tr key={r.id}>
              <td className="td">
                <Mono>{r.code}</Mono>
              </td>
              <td className="td">
                <Localized map={r.name} fallback={r.code} />
              </td>
              <td className="td">
                <div className="flex flex-wrap gap-1">
                  {r.permissions.map((p) => (
                    <span key={p} className="badge bg-slate-100 text-slate-600">
                      {p}
                    </span>
                  ))}
                </div>
              </td>
              <td className="td">
                {r.isBase ? <Pill tone="indigo">base</Pill> : "—"}
              </td>
              <td className="td text-right">
                {!r.isBase && (
                  <span className="relative inline-flex items-center gap-3">
                    <EditRole role={r} />
                    <DeleteButton
                      path={`/authorization/v1/roles/${r.id}`}
                      label="Delete"
                      confirm={`Delete role ${r.code}?`}
                    />
                  </span>
                )}
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState>No roles.</EmptyState>
      )}
      <div className="mt-4">
        <RoleCreate />
      </div>

      {/* Assignments */}
      <h2 className="mb-3 mt-8 text-sm font-semibold text-slate-900">Assignments</h2>
      <div className="mb-4 grid gap-4 sm:grid-cols-2">
        <LookupForm
          basePath="/roles"
          param="subjectPersonId"
          label="Filter by subject person"
          kind="person"
          current={subjectPersonId}
        />
        <LookupForm
          basePath="/roles"
          param="targetUnitId"
          label="Filter by target unit"
          kind="unit"
          current={targetUnitId}
        />
      </div>
      {assignments && assignments.assignments.length > 0 ? (
        <Table
          head={
            <>
              <th className="th">Subject</th>
              <th className="th">Role</th>
              <th className="th">Target unit</th>
              <th className="th">Scope</th>
              <th className="th">Expires</th>
              <th className="th"></th>
            </>
          }
        >
          {assignments.assignments.map((a) => (
            <tr key={a.id}>
              <td className="td">
                <Mono>{a.subjectPersonId.slice(-8)}</Mono>
              </td>
              <td className="td">
                <Mono>{roleById.get(a.roleId)?.code ?? a.roleId.slice(-8)}</Mono>
              </td>
              <td className="td">
                <Mono>{a.targetUnitId.slice(-8)}</Mono>
              </td>
              <td className="td">
                <Pill tone={a.scope === "subtree" ? "indigo" : "slate"}>{a.scope}</Pill>
              </td>
              <td className="td">{a.expiresAt ?? "—"}</td>
              <td className="td text-right">
                <DeleteButton
                  path={`/authorization/v1/assignments/${a.id}`}
                  label="Revoke"
                  confirm="Revoke this assignment?"
                />
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState>No assignments match.</EmptyState>
      )}
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <AssignmentGrant roles={roles?.roles ?? []} />
        <InstanceAdminGrant />
      </div>
    </div>
  );
}
