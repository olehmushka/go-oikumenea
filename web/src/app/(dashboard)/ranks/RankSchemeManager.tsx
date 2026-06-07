"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { ActionButton } from "@/components/ActionButton";
import { pickLabel } from "@/lib/i18n";
import { useLocale } from "@/lib/locale";
import type { RankScheme } from "@/lib/api/types";

/** Full add / edit / delete management of the system-wide rank scheme (category → type → rank). */
export function RankSchemeManager({ scheme }: { scheme: RankScheme }) {
  const { locale } = useLocale();
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [editing, setEditing] = useState<string | null>(null);

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

  const byOrder = <T extends { sortOrder?: number }>(a: T, b: T) =>
    (a.sortOrder ?? 0) - (b.sortOrder ?? 0);

  return (
    <div className="space-y-4">
      {err ? <ErrorBox error={err} /> : null}

      {scheme.categories.slice().sort(byOrder).map((cat) => (
        <div key={cat.id} className="card p-5">
          <div className="flex items-center justify-between gap-2">
            {editing === cat.id ? (
              <NodeEdit
                fields={[["name", "name", pickLabel(cat.name, locale)]]}
                onSave={(b) => run(() => mutate("PUT", `/rank/v1/rank-scheme/categories/${cat.id}`, b), () => setEditing(null))}
                onCancel={() => setEditing(null)}
                busy={busy}
              />
            ) : (
              <h2 className="text-sm font-semibold text-slate-900">
                {pickLabel(cat.name, locale) || cat.code}{" "}
                <span className="font-mono text-xs text-slate-400">{cat.code}</span>
              </h2>
            )}
            <span className="flex items-center gap-3">
              <EditToggle id={cat.id} editing={editing} setEditing={setEditing} />
              <ActionButton method="DELETE" path={`/rank/v1/rank-scheme/category/${cat.id}`} label="Delete" confirm={`Delete category ${cat.code}? (must have no active types)`} tone="danger" />
            </span>
          </div>

          <div className="mt-3 space-y-3">
            {cat.types?.slice().sort(byOrder).map((t) => (
              <div key={t.id} className="border-l-2 border-slate-200 pl-3">
                <div className="flex items-center justify-between gap-2">
                  {editing === t.id ? (
                    <NodeEdit
                      fields={[["name", "name", pickLabel(t.name, locale)]]}
                      onSave={(b) => run(() => mutate("PUT", `/rank/v1/rank-scheme/types/${t.id}`, b), () => setEditing(null))}
                      onCancel={() => setEditing(null)}
                      busy={busy}
                    />
                  ) : (
                    <div className="text-sm font-medium text-slate-700">
                      {pickLabel(t.name, locale) || t.code}{" "}
                      <span className="font-mono text-xs text-slate-400">{t.code}</span>
                    </div>
                  )}
                  <span className="flex items-center gap-3">
                    <EditToggle id={t.id} editing={editing} setEditing={setEditing} />
                    <ActionButton method="DELETE" path={`/rank/v1/rank-scheme/type/${t.id}`} label="Delete" confirm={`Delete type ${t.code}?`} tone="danger" />
                  </span>
                </div>

                <div className="mt-2 flex flex-wrap items-center gap-2">
                  {t.ranks?.slice().sort(byOrder).map((r) => (
                    <span key={r.id} className="badge bg-slate-100 text-slate-700">
                      {r.abbreviation ? `${r.abbreviation} · ` : ""}
                      {pickLabel(r.name, locale) || r.code}
                      <button
                        type="button"
                        className="ml-1 text-red-500 hover:text-red-700"
                        title="Delete rank"
                        onClick={() =>
                          window.confirm(`Delete rank ${r.code}?`) &&
                          run(() => mutate("DELETE", `/rank/v1/rank-scheme/rank/${r.id}`))
                        }
                      >
                        ✕
                      </button>
                    </span>
                  ))}
                  <AddInline
                    placeholders={["code", "name", "abbr"]}
                    names={["code", "name", "abbreviation"]}
                    label="+ rank"
                    busy={busy}
                    onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/ranks", { ...b, typeId: t.id }))}
                  />
                </div>
              </div>
            ))}
            <AddInline
              placeholders={["code", "name"]}
              names={["code", "name"]}
              label="+ type"
              busy={busy}
              onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/types", { ...b, categoryId: cat.id }))}
            />
          </div>
        </div>
      ))}

      <div className="card p-5">
        <h3 className="text-sm font-semibold text-slate-900">Add category</h3>
        <div className="mt-2">
          <AddInline
            placeholders={["code", "name"]}
            names={["code", "name"]}
            label="Add category"
            busy={busy}
            onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/categories", b))}
          />
        </div>
      </div>
    </div>
  );
}

function EditToggle({
  id,
  editing,
  setEditing,
}: {
  id: string;
  editing: string | null;
  setEditing: (v: string | null) => void;
}) {
  return (
    <button
      type="button"
      className="text-xs font-medium text-indigo-600 hover:underline"
      onClick={() => setEditing(editing === id ? null : id)}
    >
      {editing === id ? "Close" : "Edit"}
    </button>
  );
}

function NodeEdit({
  fields,
  onSave,
  onCancel,
  busy,
}: {
  fields: [key: string, placeholder: string, value: string][];
  onSave: (body: Record<string, string>) => void;
  onCancel: () => void;
  busy: boolean;
}) {
  return (
    <form
      className="flex items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const body: Record<string, string> = {};
        for (const [k] of fields) {
          const v = String(f.get(k) || "").trim();
          if (v) body[k] = v;
        }
        onSave(body);
      }}
    >
      {fields.map(([k, ph, val]) => (
        <input key={k} name={k} className="input" placeholder={ph} defaultValue={val} />
      ))}
      <button className="btn-primary" disabled={busy}>
        Save
      </button>
      <button type="button" className="btn-ghost" onClick={onCancel}>
        Cancel
      </button>
    </form>
  );
}

function AddInline({
  placeholders,
  names,
  label,
  busy,
  onAdd,
}: {
  placeholders: string[];
  names: string[];
  label: string;
  busy: boolean;
  onAdd: (body: Record<string, string>) => void;
}) {
  return (
    <form
      className="inline-flex items-center gap-1"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        const body: Record<string, string> = {};
        names.forEach((n) => {
          const v = String(f.get(n) || "").trim();
          if (v) body[n] = v;
        });
        if (body[names[0]] && body[names[1]]) {
          onAdd(body);
          form.reset();
        }
      }}
    >
      {names.map((n, i) => (
        <input
          key={n}
          name={n}
          required={i < 2}
          className="w-28 rounded-md border border-slate-300 bg-white px-2 py-1 text-xs outline-none focus:border-indigo-500"
          placeholder={placeholders[i]}
        />
      ))}
      <button
        className="rounded-md border border-slate-300 bg-white px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50"
        disabled={busy}
      >
        {label}
      </button>
    </form>
  );
}
