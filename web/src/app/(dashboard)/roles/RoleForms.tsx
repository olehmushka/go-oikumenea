"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { EntitySelect } from "@/components/EntitySelect";
import { GraphSelect } from "@/components/GraphSelect";
import type { Role } from "@/lib/api/types";

function useSubmit() {
  const router = useRouter();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      router.refresh();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };
  return { err, busy, run };
}

export function RoleCreate() {
  const { err, busy, run } = useSubmit();
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        run(async () => {
          await mutate("POST", "/authorization/v1/roles", {
            code: String(f.get("code") || "").trim(),
            name: String(f.get("name") || "").trim(),
            description: String(f.get("description") || "").trim() || undefined,
            permissions: String(f.get("permissions") || "")
              .split(/[\s,]+/)
              .map((s) => s.trim())
              .filter(Boolean),
          });
          form.reset();
        });
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">Create role</h3>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input name="code" required className="input" placeholder="code (e.g. unit-reader)" />
        <input name="name" required className="input" placeholder="name" />
      </div>
      <input name="description" className="input" placeholder="description (optional)" />
      <textarea
        name="permissions"
        className="input font-mono"
        rows={2}
        placeholder="permissions, space- or comma-separated (e.g. person.read unit.read)"
      />
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? "Creating…" : "Create role"}
      </button>
    </form>
  );
}

/** Inline edit of a custom role's name / description / permissions. PUT /authorization/v1/roles/{id}. */
export function EditRole({ role }: { role: Role }) {
  const { err, busy, run } = useSubmit();
  const [open, setOpen] = useState(false);
  if (!open) {
    return (
      <button
        type="button"
        className="text-xs font-medium text-indigo-600 hover:underline"
        onClick={() => setOpen(true)}
      >
        Edit
      </button>
    );
  }
  return (
    <form
      className="absolute right-0 z-20 mt-1 w-96 space-y-2 rounded-md border border-slate-200 bg-white p-3 text-left shadow-lg"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        run(async () => {
          const perms = String(f.get("permissions") || "")
            .split(/[\s,]+/)
            .map((x) => x.trim())
            .filter(Boolean);
          await mutate("PUT", `/authorization/v1/roles/${role.id}`, {
            name: String(f.get("name") || "").trim() || undefined,
            description: String(f.get("description") || "").trim() || undefined,
            permissions: perms.length ? perms : undefined,
          });
          setOpen(false);
        });
      }}
    >
      {err ? <ErrorBox error={err} /> : null}
      <input name="name" className="input" placeholder="name" defaultValue={role.code} />
      <input name="description" className="input" placeholder="description" />
      <textarea
        name="permissions"
        rows={3}
        className="input font-mono"
        defaultValue={role.permissions.join(" ")}
      />
      <div className="flex gap-2">
        <button className="btn-primary" disabled={busy}>
          Save
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          Cancel
        </button>
      </div>
    </form>
  );
}

export function AssignmentGrant({ roles }: { roles: Role[] }) {
  const { err, busy, run } = useSubmit();
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        run(async () => {
          await mutate("POST", "/authorization/v1/assignments", {
            subjectPersonId: String(f.get("subjectPersonId") || "").trim(),
            roleId: String(f.get("roleId") || "").trim(),
            targetUnitId: String(f.get("targetUnitId") || "").trim(),
            scope: String(f.get("scope") || "subtree"),
            graph: String(f.get("graph") || "").trim() || undefined,
            expiresAt: String(f.get("expiresAt") || "").trim() || undefined,
          });
          form.reset();
        });
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">Grant assignment</h3>
      <p className="text-xs text-slate-500">
        An assignment is (person, role, target unit, scope). <code>subtree</code> cascades to
        descendants; <code>unit</code> grants nothing below — not even read.
      </p>
      {err ? <ErrorBox error={err} /> : null}
      <div>
        <label className="label">Subject person</label>
        <EntitySelect name="subjectPersonId" kind="person" required placeholder="Search a person…" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <select name="roleId" required className="input" defaultValue="">
          <option value="" disabled>
            role…
          </option>
          {roles.map((r) => (
            <option key={r.id} value={r.id}>
              {r.code}
            </option>
          ))}
        </select>
        <select name="scope" className="input" defaultValue="subtree">
          <option value="subtree">subtree</option>
          <option value="unit">unit</option>
        </select>
      </div>
      <div>
        <label className="label">Target unit</label>
        <EntitySelect name="targetUnitId" kind="unit" required placeholder="Search a unit…" />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <GraphSelect name="graph" />
        <input name="expiresAt" type="datetime-local" className="input" />
      </div>
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? "Granting…" : "Grant"}
      </button>
    </form>
  );
}

export function InstanceAdminGrant() {
  const { err, busy, run } = useSubmit();
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        run(async () => {
          await mutate("POST", "/authorization/v1/instance-admins", {
            personId: String(f.get("personId") || "").trim(),
          });
          form.reset();
        });
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">Grant instance-admin</h3>
      <p className="text-xs text-slate-500">
        The instance-admin plane is separate from unit roles — it grants the whole instance.
      </p>
      {err ? <ErrorBox error={err} /> : null}
      <div className="flex items-start gap-2">
        <div className="flex-1">
          <EntitySelect name="personId" kind="person" required placeholder="Search a person…" />
        </div>
        <button type="submit" className="btn-primary" disabled={busy}>
          {busy ? "…" : "Grant"}
        </button>
      </div>
    </form>
  );
}
