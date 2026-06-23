"use client";

// AccountManager — bind a person to a login ACCOUNT and its external identities (M8 /
// identity-federation). A person is account-optional: this surfaces whether they have one and lets an
// operator create it, federate Keycloak login points (issuer + subject), unlink them, and disable
// login. The backend (identity-federation) owns the constraints (≤1 active account per person; an
// (issuer, subject) is globally unique). issuer = the realm issuer URL (token `iss`); subject = the
// Keycloak user's `sub` claim (the user UUID) — NOT the username. Mirrors the EmailManager pattern.

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { isConjureError } from "oikumenea-client";
import { ErrorBox } from "@/components/ErrorBox";
import { Pill } from "@/components/ui";
import { T } from "@/components/T";

type ExternalIdentity = { id: string; accountId: string; issuer: string; subject: string; createdAt?: string };
type Account = { id: string; personId: string; email?: string; status: string; identities?: ExternalIdentity[] };
type IssuerOption = { issuer: string; audience?: string; type: string };

const tail = (id: string) => id.slice(-8);

// IssuerField renders a dropdown of the instance's configured issuers when any are known, and falls
// back to a free-text input otherwise (e.g. before config loads, or an instance with no issuers).
function IssuerField({ issuers, required }: { issuers: IssuerOption[]; required?: boolean }) {
  if (issuers.length === 0) {
    return (
      <input
        name="issuer"
        required={required}
        className="input w-full"
        placeholder="issuer — realm URL (token `iss`)"
      />
    );
  }
  return (
    <select name="issuer" required={required} className="input w-full" defaultValue={issuers[0].issuer}>
      {issuers.map((i) => (
        <option key={i.issuer} value={i.issuer}>
          {i.issuer}
          {i.type === "hs256" ? " (dev)" : ""}
        </option>
      ))}
    </select>
  );
}

export function AccountManager({ personId }: { personId: string }) {
  const router = useRouter();
  const [account, setAccount] = useState<Account | null>(null);
  const [issuers, setIssuers] = useState<IssuerOption[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);

  // Read the person's active account; a 404 means roster-only (no account) — not an error.
  const load = useCallback(async () => {
    try {
      setAccount(await api.request<Account>("GET", `/identity/v1/persons/${personId}/account`));
      setErr(null);
    } catch (e) {
      if (isConjureError(e) && e.status === 404) setAccount(null);
      else setErr(e);
    } finally {
      setLoaded(true);
    }
  }, [personId]);

  useEffect(() => {
    load();
  }, [load]);

  // The configured issuers are static instance config; fetch once for the dropdown. A failure just
  // leaves the field as free text — non-fatal.
  useEffect(() => {
    api
      .request<unknown>("GET", "/identity/v1/issuers")
      .then((rows) => setIssuers(Array.isArray(rows) ? rows : []))
      .catch(() => setIssuers([]));
  }, []);

  // run a mutation, then refresh both this card's account view and the server-rendered page.
  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      after?.();
      await load();
      router.refresh();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  if (!loaded) return <p className="mt-2 text-sm text-slate-400"><T>Loading…</T></p>;

  return (
    <div className="mt-2 space-y-3">
      {err ? <ErrorBox error={err} /> : null}
      {account ? (
        <ExistingAccount account={account} issuers={issuers} busy={busy} run={run} />
      ) : (
        <CreateAccount personId={personId} issuers={issuers} busy={busy} run={run} />
      )}
    </div>
  );
}

function ExistingAccount({
  account,
  issuers,
  busy,
  run,
}: {
  account: Account;
  issuers: IssuerOption[];
  busy: boolean;
  run: (fn: () => Promise<unknown>, after?: () => void) => Promise<void>;
}) {
  const identities = account.identities ?? [];
  return (
    <div className="space-y-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Pill tone={account.status === "active" ? "green" : "slate"}>{account.status}</Pill>
          <span className="text-slate-700">{account.email || <span className="text-slate-400"><T>no email</T></span>}</span>
        </div>
        {account.status === "active" ? (
          <button
            type="button"
            className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
            disabled={busy}
            onClick={() =>
              window.confirm("Disable login on this account?") &&
              run(() => api.identityFederation.disableAccount(account.id))
            }
          >
            <T>Disable login</T>
          </button>
        ) : null}
      </div>

      <div>
        <div className="text-xs font-medium uppercase tracking-wide text-slate-400"><T>Login identities</T></div>
        {identities.length === 0 ? (
          <p className="mt-1 text-sm text-slate-400"><T>No linked identities — this account cannot log in yet.</T></p>
        ) : (
          <ul className="mt-1 space-y-0.5 text-sm text-slate-700">
            {identities.map((i) => (
              <li key={i.id} className="flex items-center justify-between gap-2">
                <span className="truncate">
                  <span className="text-slate-500">{i.issuer}</span> · <span className="font-mono">{tail(i.subject)}</span>
                </span>
                <button
                  type="button"
                  className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
                  disabled={busy}
                  onClick={() =>
                    window.confirm("Unlink this login identity?") &&
                    run(() => api.identityFederation.unlinkIdentity(account.id, i.id))
                  }
                >
                  <T>Unlink</T>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      <IdentityForm
        issuers={issuers}
        busy={busy}
        onSubmit={(issuer, subject, reset) =>
          run(() => api.identityFederation.linkIdentity(account.id, { issuer, subject }), reset)
        }
      />
    </div>
  );
}

function CreateAccount({
  personId,
  issuers,
  busy,
  run,
}: {
  personId: string;
  issuers: IssuerOption[];
  busy: boolean;
  run: (fn: () => Promise<unknown>, after?: () => void) => Promise<void>;
}) {
  return (
    <div className="space-y-2 text-sm">
      <p className="text-slate-400"><T>This person has no login account yet.</T></p>
      <form
        className="space-y-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          const email = String(f.get("email") || "").trim();
          const issuer = String(f.get("issuer") || "").trim();
          const subject = String(f.get("subject") || "").trim();
          const body: { personId: string; email?: string; identity?: { issuer: string; subject: string } } = { personId };
          if (email) body.email = email;
          if (issuer && subject) body.identity = { issuer, subject };
          run(() => api.identityFederation.createAccount(body), () => form.reset());
        }}
      >
        <input name="email" type="email" className="input w-full" placeholder="email@example.com (optional)" />
        <IssuerField issuers={issuers} />
        <input name="subject" className="input w-full" placeholder="subject — Keycloak user `sub` UUID" />
        <button className="btn-primary" disabled={busy}>
          <T>Create login account</T>
        </button>
      </form>
      <p className="text-xs text-slate-400">
        <T>Leave issuer/subject blank to create a login-less shell; you can link a Keycloak identity afterwards.</T>
      </p>
    </div>
  );
}

function IdentityForm({
  issuers,
  busy,
  onSubmit,
}: {
  issuers: IssuerOption[];
  busy: boolean;
  onSubmit: (issuer: string, subject: string, reset: () => void) => void;
}) {
  return (
    <form
      className="space-y-2 border-t border-slate-100 pt-3"
      onSubmit={(ev) => {
        ev.preventDefault();
        const f = new FormData(ev.currentTarget);
        const form = ev.currentTarget;
        const issuer = String(f.get("issuer") || "").trim();
        const subject = String(f.get("subject") || "").trim();
        if (!issuer || !subject) return;
        onSubmit(issuer, subject, () => form.reset());
      }}
    >
      <div className="text-xs font-medium uppercase tracking-wide text-slate-400"><T>Link Keycloak identity</T></div>
      <IssuerField issuers={issuers} required />
      <input name="subject" required className="input w-full" placeholder="subject — Keycloak user `sub` UUID" />
      <button className="btn-ghost" disabled={busy}>
        <T>Link identity</T>
      </button>
    </form>
  );
}
