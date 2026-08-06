import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
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
import { T } from "@/components/T";
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
  // Assignments are scoped reads: the API requires *exactly one* of subjectPersonId/targetUnitId,
  // so there is no unconditional global list. Only query when one (and only one) filter is set.
  const assignmentFilter = subjectPersonId && !targetUnitId
    ? (["subjectPersonId", subjectPersonId] as const)
    : targetUnitId && !subjectPersonId
      ? (["targetUnitId", targetUnitId] as const)
      : null;
  let roles: RolePage | null = null;
  let assignments: AssignmentPage | null = null;
  let error: unknown = null;
  let assignmentError: unknown = null;
  try {
    roles = await oikumenea().then((ok) => ok.authorization.listRoles(100));
  } catch (e) {
    error = e;
  }
  if (assignmentFilter) {
    try {
      const ok = await oikumenea();
      // Positional SDK args, and the endpoint gained three filters between targetUnitId and pageSize
      // in M58 ticket 6 (roleId / scope / graphId) — hence the explicit undefineds. They are spelled
      // out rather than dropped because a positional call that silently shifts is how `50` became a
      // roleId: the compiler caught it here, and the /audit page's version of this same mistake is
      // what ticket 1 retired.
      assignments = await ok.authorization.listAssignments(
        assignmentFilter[0] === "subjectPersonId" ? assignmentFilter[1] : undefined,
        assignmentFilter[0] === "targetUnitId" ? assignmentFilter[1] : undefined,
        undefined, // roleId — the explorer's filter, not this page's
        undefined, // scope
        undefined, // graphId
        50,
      );
    } catch (e) {
      assignmentError = e;
    }
  }

  const roleById = new Map((roles?.roles ?? []).map((r) => [r.id, r]));

  return (
    <div>
      <PageHeader
        title={<T>Roles &amp; access</T>}
        description={<T>RBAC: code-defined permissions packaged into roles, then granted as scoped assignments. Authority comes only from assignments — never rank or position.</T>}
      />
      {error ? <ErrorNotice error={error} /> : null}

      {/* Roles */}
      <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>Roles</T></h2>
      {roles && roles.roles.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Code</T></th>
              <th className="th"><T>Name</T></th>
              <th className="th"><T>Permissions</T></th>
              <th className="th"><T>Base</T></th>
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
                {r.isBase ? <Pill tone="indigo"><T>base</T></Pill> : "—"}
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
        <EmptyState><T>No roles.</T></EmptyState>
      )}
      <div className="mt-4">
        <RoleCreate />
      </div>

      {/* Assignments */}
      <div className="mb-3 mt-8 flex items-center gap-3">
        <h2 className="text-sm font-semibold text-slate-900"><T>Assignments</T></h2>
        <Link href="/explore/link__has_role" className="ml-auto text-xs text-indigo-600 hover:underline">
          <T>Browse, filter and chart every grant →</T>
        </Link>
      </div>
      <p className="mb-4 text-xs text-slate-500">
        <T>This is the GRANT surface: pick a subject or a unit to see what to revoke, and grant below. Browsing the whole grant population — filtered by role, scope or cascade graph, with charts over the same filters — is the explorer&apos;s job since M58 ticket 6.</T>
      </p>
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
      {assignmentError ? <ErrorNotice error={assignmentError} /> : null}
      {!assignmentFilter ? (
        <EmptyState>
          <T>Pick a subject person or a target unit to list the grants you might revoke — or browse them all in the explorer.</T>
        </EmptyState>
      ) : assignments && assignments.assignments.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Subject</T></th>
              <th className="th"><T>Role</T></th>
              <th className="th"><T>Target unit</T></th>
              <th className="th"><T>Scope</T></th>
              <th className="th"><T>Expires</T></th>
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
        <EmptyState><T>No assignments match.</T></EmptyState>
      )}
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <AssignmentGrant roles={roles?.roles ?? []} />
        <InstanceAdminGrant />
      </div>
    </div>
  );
}
