"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { Localized } from "@/components/Localized";
import type { DocumentType, PersonalCodeScheme } from "@/lib/api/types";

function useRun() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const run = (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    fn()
      .then(() => {
        after?.();
        router.refresh();
      })
      .catch(setErr)
      .finally(() => setBusy(false));
  };
  return { busy, err, run };
}

/** Create / edit (rename, retire) document types. No hard delete — status flips to retired. */
export function DocTypeManager({ types }: { types: DocumentType[] }) {
  const { busy, err, run } = useRun();
  const [editing, setEditing] = useState<string | null>(null);
  return (
    <div className="space-y-2">
      {err ? <ErrorBox error={err} /> : null}
      {types.map((t) =>
        editing === t.id ? (
          <form
            key={t.id}
            className="card flex items-center gap-2 p-3"
            onSubmit={(e) => {
              e.preventDefault();
              const f = new FormData(e.currentTarget);
              run(
                () =>
                  mutate("PUT", `/document/v1/document-types/${t.id}`, {
                    name: String(f.get("name") || "").trim() || undefined,
                    status: String(f.get("status") || "").trim() || undefined,
                  }),
                () => setEditing(null),
              );
            }}
          >
            <input name="name" className="input" defaultValue={t.code} placeholder="name" />
            <select name="status" className="input" defaultValue={t.status ?? "active"}>
              <option value="active">active</option>
              <option value="retired">retired</option>
            </select>
            <button className="btn-primary" disabled={busy}>
              Save
            </button>
            <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
              Cancel
            </button>
          </form>
        ) : (
          <div key={t.id} className="card flex items-center justify-between p-3">
            <div>
              <div className="text-sm font-medium text-slate-800">
                <Localized map={t.name} fallback={t.code} />
              </div>
              <span className="font-mono text-xs text-slate-400">{t.code}</span>
            </div>
            <span className="flex items-center gap-3">
              <span className="text-xs text-slate-400">{t.status}</span>
              <button
                type="button"
                className="text-xs font-medium text-indigo-600 hover:underline"
                onClick={() => setEditing(t.id)}
              >
                Edit
              </button>
            </span>
          </div>
        ),
      )}
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const form = e.currentTarget;
          run(
            () =>
              mutate("POST", "/document/v1/document-types", {
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim(),
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="code" required className="input" placeholder="code" />
        <input name="name" required className="input" placeholder="name" />
        <button className="btn-ghost" disabled={busy}>
          Add type
        </button>
      </form>
    </div>
  );
}

/** Create / edit (rename, retire, set regex) personal-code schemes. Keyed by code. */
export function SchemeManager({ schemes }: { schemes: PersonalCodeScheme[] }) {
  const { busy, err, run } = useRun();
  const [editing, setEditing] = useState<string | null>(null);
  return (
    <div className="space-y-2">
      {err ? <ErrorBox error={err} /> : null}
      {schemes.map((s) =>
        editing === s.code ? (
          <form
            key={s.code}
            className="card flex items-center gap-2 p-3"
            onSubmit={(e) => {
              e.preventDefault();
              const f = new FormData(e.currentTarget);
              run(
                () =>
                  mutate("PUT", `/document/v1/personal-code-schemes/${s.code}`, {
                    name: String(f.get("name") || "").trim() || undefined,
                    validationRegex: String(f.get("validationRegex") || "").trim() || undefined,
                    status: String(f.get("status") || "").trim() || undefined,
                  }),
                () => setEditing(null),
              );
            }}
          >
            <input name="name" className="input" defaultValue={s.code} placeholder="name" />
            <input name="validationRegex" className="input font-mono" placeholder="regex (optional)" />
            <select name="status" className="input" defaultValue={s.status ?? "active"}>
              <option value="active">active</option>
              <option value="retired">retired</option>
            </select>
            <button className="btn-primary" disabled={busy}>
              Save
            </button>
            <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
              Cancel
            </button>
          </form>
        ) : (
          <div key={s.code} className="card flex items-center justify-between p-3">
            <div>
              <div className="text-sm font-medium text-slate-800">
                <Localized map={s.name} fallback={s.code} />
              </div>
              <span className="font-mono text-xs text-slate-400">{s.code}</span>
            </div>
            <span className="flex items-center gap-3">
              {s.country ? <span className="text-xs text-slate-400">{s.country}</span> : null}
              <span className="text-xs text-slate-400">{s.status}</span>
              <button
                type="button"
                className="text-xs font-medium text-indigo-600 hover:underline"
                onClick={() => setEditing(s.code)}
              >
                Edit
              </button>
            </span>
          </div>
        ),
      )}
      <form
        className="grid grid-cols-2 gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const f = new FormData(e.currentTarget);
          const form = e.currentTarget;
          run(
            () =>
              mutate("POST", "/document/v1/personal-code-schemes", {
                code: String(f.get("code") || "").trim(),
                name: String(f.get("name") || "").trim(),
                genericCategory: String(f.get("genericCategory") || "").trim(),
                countryIso: String(f.get("countryIso") || "").trim() || undefined,
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="code" required className="input" placeholder="code (e.g. ua-rnokpp)" />
        <input name="name" required className="input" placeholder="name" />
        <input name="genericCategory" required className="input" placeholder="category (e.g. tax-id)" />
        <input name="countryIso" className="input" placeholder="country ISO (optional)" />
        <button className="btn-ghost col-span-2" disabled={busy}>
          Add scheme
        </button>
      </form>
    </div>
  );
}
