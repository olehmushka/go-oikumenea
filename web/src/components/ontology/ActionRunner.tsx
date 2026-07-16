"use client";

// The generic action runner (D-ActionInvocation, review-2026-09 R-33): turns the read-only Actions card
// on /o/[rid] operational. Each action carries its endpoint binding (method/path/pathParams) + parameter
// schema, both single-sourced from the contract, so this renders a form and POSTs via the SDK's generic
// request() with no hand-authored URL. Phase 2 handles object-level actions with a flat body (the single
// path param, if any, is the object's own RID); structured bodies (Phase 3) and sub-resource actions
// (Phase 4) extend from here.

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { ActionParamsList } from "@/components/ActionParamsList";
import { T } from "@/components/T";
import type { ActionType } from "@/lib/api/types";
import {
  isObjectLevelInvocable,
  isInvocable,
  buildPath,
  buildBody,
  inputKind,
  fieldKind,
} from "@/lib/ontology/actions";
import { isMasked, validateSensitive } from "@/lib/ontology/sensitive";
import type { ActionParam } from "@/lib/api/types";

export function ActionRunner({ actions, rid }: { actions: ActionType[]; rid: string }) {
  return (
    <ul className="divide-y divide-slate-100">
      {actions.map((a) => (
        <ActionRow key={a.code} action={a} rid={rid} />
      ))}
    </ul>
  );
}

function ActionRow({ action, rid }: { action: ActionType; rid: string }) {
  const [open, setOpen] = useState(false);
  const runnable = isObjectLevelInvocable(action);
  const method = action.endpoint?.method;

  return (
    <li className="py-2">
      <div className="flex items-center gap-2">
        <code className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-700">{action.code}</code>
        {method ? (
          <span className={`rounded px-1 py-0.5 text-[10px] font-semibold ${methodClass(method)}`}>{method}</span>
        ) : null}
        <span className="text-xs text-slate-400">{action.permission}</span>
        <div className="ml-auto">
          {runnable ? (
            <button
              type="button"
              className="text-xs font-medium text-indigo-600 hover:underline"
              onClick={() => setOpen((v) => !v)}
            >
              {open ? <T>Close</T> : <T>Run…</T>}
            </button>
          ) : isInvocable(action) ? (
            <span className="text-xs text-slate-400" title="Structured body or sub-resource — run from the object's dedicated form">
              <T>run from form</T>
            </span>
          ) : (
            <span className="text-xs text-slate-300" title="No invocable endpoint (internal or bulk-ingestion action)">
              <T>not runnable</T>
            </span>
          )}
        </div>
      </div>
      {open && runnable ? (
        <RunForm action={action} rid={rid} onClose={() => setOpen(false)} />
      ) : (
        <div className="mt-1">
          <ActionParamsList params={action.parameters} />
        </div>
      )}
    </li>
  );
}

function RunForm({ action, rid, onClose }: { action: ActionType; rid: string; onClose: () => void }) {
  const router = useRouter();
  const endpoint = action.endpoint!;
  const params = action.parameters ?? [];
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [done, setDone] = useState(false);

  const set = (k: string, v: unknown) => setValues((prev) => ({ ...prev, [k]: v }));

  // The single path param (when present) is the object's own RID; preview the concrete URL the run hits.
  const pathVals: Record<string, string> = {};
  const pp = endpoint.pathParams ?? [];
  if (pp.length === 1) pathVals[pp[0]] = rid;
  const preview = buildPath(endpoint, pathVals);

  async function onSubmit(ev: React.FormEvent<HTMLFormElement>) {
    ev.preventDefault();
    // Advisory sensitive-field validation (server is authoritative, but fail fast on an obvious bad PAN/IBAN).
    for (const p of params) {
      const msg = validateSensitive(p.sensitivity, String(values[p.name] ?? ""));
      if (msg) {
        setErr(new Error(`${p.name}: ${msg}`));
        return;
      }
    }
    if (isDestructive(action.code, endpoint.method) && !window.confirm(`Run ${action.code}? This cannot be undone.`)) return;
    setBusy(true);
    setErr(null);
    try {
      const opts = params.length > 0 ? { body: buildBody(params, values) } : undefined;
      await api.request(endpoint.method, preview, opts);
      setDone(true);
      router.refresh(); // re-fetch the server component: Properties + History reflect the change
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="mt-2 rounded-md border border-slate-200 bg-slate-50 p-3">
      <div className="mb-2 flex items-center gap-2 text-[11px] text-slate-500">
        <span className={`rounded px-1 py-0.5 font-semibold ${methodClass(endpoint.method)}`}>{endpoint.method}</span>
        <code className="font-mono">{preview}</code>
      </div>

      {params.map((p) => (
        <div key={p.name} className="mb-2">
          <label className="label flex items-center gap-1">
            <code className="font-mono text-xs">{p.name}</code>
            {p.required ? <span className="text-amber-600" title="required">*</span> : null}
            <span className="text-xs text-slate-400">: {p.type}</span>
          </label>
          <Field param={p} value={values[p.name]} onChange={(v) => set(p.name, v)} />
          {p.docs ? <p className="mt-0.5 text-[11px] text-slate-400">{p.docs}</p> : null}
        </div>
      ))}

      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      {done && !err ? (
        <div className="mb-2 rounded border border-green-200 bg-green-50 px-2 py-1 text-xs text-green-800">
          <T>Done — the object history below reflects the change.</T>
        </div>
      ) : null}

      <div className="flex items-center gap-3">
        <button type="submit" className="btn" disabled={busy}>
          {busy ? <T>Running…</T> : <T>Run action</T>}
        </button>
        <button type="button" className="text-xs text-slate-500 hover:underline" onClick={onClose}>
          <T>Cancel</T>
        </button>
      </div>
    </form>
  );
}

/** Renders one request-body param by its kind: a flat scalar input, a repeatable list of scalar inputs,
 * or a raw-JSON editor for the deep shapes (nested objects, list-of-object, map). */
function Field({ param, value, onChange }: { param: ActionParam; value: unknown; onChange: (v: unknown) => void }) {
  const kind = fieldKind(param.type);

  if (kind === "list") {
    const items = (value as string[] | undefined) ?? [""];
    return (
      <div className="space-y-1">
        {items.map((it, i) => (
          <div key={i} className="flex gap-1">
            <input
              className="input"
              value={it}
              onChange={(e) => onChange(items.map((x, j) => (j === i ? e.target.value : x)))}
            />
            <button
              type="button"
              className="px-1 text-xs text-slate-400 hover:text-red-600"
              title="remove"
              onClick={() => onChange(items.filter((_, j) => j !== i))}
            >
              ×
            </button>
          </div>
        ))}
        <button
          type="button"
          className="text-xs text-indigo-600 hover:underline"
          onClick={() => onChange([...items, ""])}
        >
          <T>+ add</T>
        </button>
      </div>
    );
  }

  if (kind === "json") {
    const skeleton = /^(list|set)</.test(param.type) ? "[]" : "{}";
    return (
      <textarea
        className="input font-mono text-xs"
        rows={4}
        placeholder={skeleton}
        value={(value as string) ?? ""}
        required={param.required}
        onChange={(e) => onChange(e.target.value)}
      />
    );
  }

  const ik = inputKind(param.type);
  if (ik === "checkbox") {
    return (
      <input
        type="checkbox"
        checked={value === true || value === "true"}
        onChange={(e) => onChange(e.target.checked)}
      />
    );
  }
  const masked = isMasked(param.sensitivity);
  const advisory = validateSensitive(param.sensitivity, String(value ?? ""));
  return (
    <div>
      <div className="flex items-center gap-1">
        {param.sensitivity ? <span title={`sensitive: ${param.sensitivity}`} className="text-amber-500">🔒</span> : null}
        <input
          className="input"
          type={masked ? "password" : ik}
          autoComplete={masked ? "off" : undefined}
          value={(value as string) ?? ""}
          required={param.required}
          onChange={(e) => onChange(e.target.value)}
        />
      </div>
      {advisory ? <p className="mt-0.5 text-[11px] text-red-600">{advisory}</p> : null}
    </div>
  );
}

// isDestructive gates the confirm dialog: any DELETE, plus POST/PUT lifecycle actions whose leaf verb
// removes or tears down (purge/erase/deactivate/disable/abolish/revoke/close/end) — a purge is a POST but
// no less irreversible than a delete.
function isDestructive(code: string, method: string): boolean {
  if (method === "DELETE") return true;
  const verb = code.split(".").pop() ?? "";
  return /^(purge|erase|deactivate|disable|abolish|revoke|close|end|remove|delete)$/.test(verb);
}

function methodClass(method: string): string {
  switch (method) {
    case "POST":
      return "bg-green-100 text-green-700";
    case "PUT":
      return "bg-amber-100 text-amber-700";
    case "DELETE":
      return "bg-red-100 text-red-700";
    default:
      return "bg-slate-100 text-slate-600";
  }
}
