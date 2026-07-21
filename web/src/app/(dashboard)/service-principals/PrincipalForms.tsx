"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import type { IssuerOption, ServicePrincipal } from "@/lib/api/types";

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

/**
 * Register a machine subject. `issuer` is offered as a select over the instance's configured issuers
 * — a token whose `iss` is not among them can never validate, so free text here would only produce
 * principals that silently never authenticate.
 *
 * Registering creates NO credential: the IdP owns the client secret (L-AuthzOnly). `subject` is the
 * `sub` of the client-credentials token, which for most IdPs is the client's service-account id.
 */
export function PrincipalRegister({ issuers }: { issuers: IssuerOption[] }) {
  const { err, busy, run } = useSubmit();
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        run(async () => {
          await api.identityFederation.registerServicePrincipal({
            code: String(f.get("code") || "").trim(),
            name: String(f.get("name") || "").trim(),
            description: String(f.get("description") || "").trim() || undefined,
            issuer: String(f.get("issuer") || "").trim(),
            subject: String(f.get("subject") || "").trim(),
            clientId: String(f.get("clientId") || "").trim() || undefined,
          });
          form.reset();
        });
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">Register service principal</h3>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input name="code" required className="input" placeholder="code (e.g. hr-connector)" />
        <input name="name" required className="input" placeholder="name" />
      </div>
      <input name="description" className="input" placeholder="description (optional)" />
      {issuers.length > 0 ? (
        <select name="issuer" required className="input" defaultValue="">
          <option value="" disabled>
            issuer (the token&rsquo;s `iss`)
          </option>
          {issuers.map((i) => (
            <option key={i.issuer} value={i.issuer}>
              {i.issuer}
            </option>
          ))}
        </select>
      ) : (
        <input name="issuer" required className="input font-mono" placeholder="issuer (the token's `iss`)" />
      )}
      <div className="grid grid-cols-2 gap-3">
        <input
          name="subject"
          required
          className="input font-mono"
          placeholder="subject (the token's `sub`) — immutable"
        />
        <input name="clientId" className="input font-mono" placeholder="clientId / azp (display only)" />
      </div>
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? "Registering…" : "Register principal"}
      </button>
      <p className="text-xs text-slate-400">
        Creates no credential — the IdP owns the client secret. (issuer, subject) is the identity key
        and is immutable; to re-point a machine, register a new principal and disable this one.
      </p>
    </form>
  );
}

/** Edit the display fields only — (issuer, subject) is immutable by contract. */
export function EditPrincipal({ principal }: { principal: ServicePrincipal }) {
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
          await api.identityFederation.updateServicePrincipal(principal.id, {
            name: String(f.get("name") || "").trim(),
            description: String(f.get("description") || "").trim() || undefined,
            clientId: String(f.get("clientId") || "").trim() || undefined,
          });
          setOpen(false);
        });
      }}
    >
      {err ? <ErrorBox error={err} /> : null}
      <input name="name" required defaultValue={principal.name} className="input" placeholder="name" />
      <input
        name="description"
        defaultValue={principal.description ?? ""}
        className="input"
        placeholder="description (optional)"
      />
      <input
        name="clientId"
        defaultValue={principal.clientId ?? ""}
        className="input font-mono"
        placeholder="clientId (display only)"
      />
      <div className="flex gap-2">
        <button type="submit" className="btn-primary" disabled={busy}>
          {busy ? "Saving…" : "Save"}
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          Cancel
        </button>
      </div>
    </form>
  );
}

/**
 * Disable/enable is the kill switch — there is no delete, so audit rows naming the principal stay
 * intact. A disabled principal fails resolution, so its tokens stop working at once.
 */
export function PrincipalStatusToggle({ principal }: { principal: ServicePrincipal }) {
  const { err, busy, run } = useSubmit();
  const disabled = principal.status === "disabled";
  return (
    <span className="inline-flex items-center gap-2">
      {err ? <ErrorBox error={err} /> : null}
      <button
        type="button"
        className="text-xs font-medium text-indigo-600 hover:underline disabled:opacity-50"
        disabled={busy}
        onClick={() => {
          if (
            !disabled &&
            !window.confirm(
              `Disable ${principal.code}? Its tokens stop working immediately.`,
            )
          ) {
            return;
          }
          run(() =>
            disabled
              ? api.identityFederation.enableServicePrincipal(principal.id)
              : api.identityFederation.disableServicePrincipal(principal.id),
          );
        }}
      >
        {busy ? "…" : disabled ? "Enable" : "Disable"}
      </button>
    </span>
  );
}

/**
 * Grant one permission code to a principal. Flat by construction (D-ServiceIdentities as amended in
 * M51): no target unit, no scope, no graph — a machine has no unit reach. An empty organization means
 * INSTANCE-WIDE; naming one confines the principal to that organization's data.
 */
export function GrantPermission({ principalId }: { principalId: string }) {
  const { err, busy, run } = useSubmit();
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        run(async () => {
          await api.authorization.grantPrincipalPermission({
            principalId,
            permission: String(f.get("permission") || "").trim(),
            orgId: String(f.get("orgId") || "").trim() || undefined,
          });
          form.reset();
        });
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">Grant permission</h3>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input
          name="permission"
          required
          className="input font-mono"
          placeholder="permission code (e.g. import.manage)"
        />
        <input
          name="orgId"
          className="input font-mono"
          placeholder="organization RID (blank = instance-wide)"
        />
      </div>
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? "Granting…" : "Grant"}
      </button>
    </form>
  );
}

/** Revoke a grant. A flip, not a delete — the row survives with `revokedAt` set. */
export function RevokeGrant({ grantId, permission }: { grantId: string; permission: string }) {
  const { err, busy, run } = useSubmit();
  return (
    <span className="inline-flex items-center gap-2">
      {err ? <ErrorBox error={err} /> : null}
      <button
        type="button"
        className="text-xs font-medium text-rose-600 hover:underline disabled:opacity-50"
        disabled={busy}
        onClick={() => {
          if (!window.confirm(`Revoke ${permission}?`)) return;
          run(() => api.authorization.revokePrincipalPermission(grantId));
        }}
      >
        {busy ? "…" : "Revoke"}
      </button>
    </span>
  );
}
