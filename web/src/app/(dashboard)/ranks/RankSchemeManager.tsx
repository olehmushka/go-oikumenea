"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { ActionButton } from "@/components/ActionButton";
import { pickLabel } from "@/lib/i18n";
import { useLocale } from "@/lib/locale";
import type { RankCategory, RankGrade, RankScheme, RankSystem, RankType } from "@/lib/api/types";

type Ordered = { id: string; sortOrder?: number };
type Option = { value: string; label: string };
type Field = {
  name: string;
  placeholder: string;
  type?: "text" | "number";
  value?: string;
  options?: Option[]; // when present, renders a <select> (e.g. the STANAG grade picker)
};

// `sortOrder` is the seniority/priority ordinal (lower = more senior) within a node's siblings — the
// only "level/priority" the rank scheme carries (D-Rank). This editor sets it explicitly (a number box
// per node + on the add forms) and via ▲▼ nudges.

/** Full add / edit / delete / reorder of the system-wide rank scheme: a tree of systems → categories →
 * types (a type may have sub-types) → ranks on the leaf types. Ranks may carry a NATO STANAG-2116
 * grade code for cross-system equivalence. */
export function RankSchemeManager({ scheme, grades }: { scheme: RankScheme; grades: RankGrade[] }) {
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

  const byOrder = <T extends Ordered>(a: T, b: T) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0);

  // Nudge an item up/down by renumbering its siblings to a clean 0..n-1 order (robust even when stored
  // priorities are sparse or tied). Only writes the rows that actually changed.
  const reorder = <T extends Ordered>(
    siblings: T[],
    item: T,
    dir: -1 | 1,
    putPath: (id: string) => string,
  ) => {
    const sorted = [...siblings].sort(byOrder);
    const i = sorted.findIndex((x) => x.id === item.id);
    const j = i + dir;
    if (i < 0 || j < 0 || j >= sorted.length) return;
    [sorted[i], sorted[j]] = [sorted[j], sorted[i]];
    const writes = sorted
      .map((x, idx) => ((x.sortOrder ?? idx) === idx ? null : mutate("PUT", putPath(x.id), { sortOrder: idx })))
      .filter((p): p is Promise<unknown> => p !== null);
    if (writes.length) run(() => Promise.all(writes));
  };

  const setPriority = (putPath: string, n: number) => run(() => mutate("PUT", putPath, { sortOrder: n }));

  // STANAG grade options for the rank forms (junior → senior within tier).
  const gradeOptions: Option[] = [
    { value: "", label: "grade…" },
    ...grades.map((g) => ({ value: g.code, label: `${g.code} — ${g.tier}` })),
  ];

  // A type renders recursively: types form a tree (a type may have child types). Ranks live on LEAF
  // types only, so a type with children shows no ranks (and offers "Add child type"), while a type
  // with ranks offers "Add rank"; an empty type offers both. `siblings`/`ti` drive reordering within
  // the parent group.
  const renderType = (t: RankType, siblings: RankType[], ti: number) => {
    const children = (t.children ?? []).slice().sort(byOrder);
    const ranks = (t.ranks ?? []).slice().sort(byOrder);
    const typePath = `/rank/v1/rank-scheme/types/${t.id}`;
    const canHoldRanks = children.length === 0; // leaf (or empty) types hold ranks
    const canAddChild = ranks.length === 0; // a type with ranks cannot also gain children
    return (
      <div key={t.id} className="border-l-2 border-slate-200 pl-3">
        <div className="flex items-center justify-between gap-2">
          {editing === t.id ? (
            <NodeEdit
              fields={[
                { name: "name", placeholder: "name", value: pickLabel(t.name, locale) },
                { name: "sortOrder", placeholder: "priority", type: "number", value: numStr(t.sortOrder) },
              ]}
              onSave={(b) => run(() => mutate("PUT", typePath, b), () => setEditing(null))}
              onCancel={() => setEditing(null)}
              busy={busy}
            />
          ) : (
            <div className="flex items-center gap-2 text-sm font-medium text-slate-700">
              <PriorityChip value={t.sortOrder} busy={busy} onSet={(n) => setPriority(typePath, n)} />
              {pickLabel(t.name, locale) || t.code}{" "}
              <span className="font-mono text-xs text-slate-400">{t.code}</span>
            </div>
          )}
          <span className="flex items-center gap-3">
            <Reorder
              busy={busy}
              upDisabled={ti === 0}
              downDisabled={ti === siblings.length - 1}
              onUp={() => reorder(siblings, t, -1, (id) => `/rank/v1/rank-scheme/types/${id}`)}
              onDown={() => reorder(siblings, t, 1, (id) => `/rank/v1/rank-scheme/types/${id}`)}
            />
            <EditToggle id={t.id} editing={editing} setEditing={setEditing} />
            <ActionButton
              method="DELETE"
              path={`/rank/v1/rank-scheme/type/${t.id}`}
              label="Delete"
              confirm={`Delete type ${t.code}? (must have no active child types or ranks)`}
              tone="danger"
            />
          </span>
        </div>

        <div className="mt-2 space-y-1">
          {children.map((c, ci) => renderType(c, children, ci))}

          {canHoldRanks &&
            ranks.map((r, ri) => {
              const rankPath = `/rank/v1/rank-scheme/ranks/${r.id}`;
              return editing === r.id ? (
                <NodeEdit
                  key={r.id}
                  fields={[
                    { name: "name", placeholder: "name", value: pickLabel(r.name, locale) },
                    { name: "abbreviation", placeholder: "abbr", value: r.abbreviation ?? "" },
                    { name: "gradeCode", placeholder: "grade", value: r.gradeCode ?? "", options: gradeOptions },
                    { name: "sortOrder", placeholder: "priority", type: "number", value: numStr(r.sortOrder) },
                  ]}
                  onSave={(b) => run(() => mutate("PUT", rankPath, b), () => setEditing(null))}
                  onCancel={() => setEditing(null)}
                  busy={busy}
                />
              ) : (
                <div key={r.id} className="flex items-center gap-2 text-sm text-slate-700">
                  <PriorityChip value={r.sortOrder} busy={busy} onSet={(n) => setPriority(rankPath, n)} />
                  <Reorder
                    busy={busy}
                    upDisabled={ri === 0}
                    downDisabled={ri === ranks.length - 1}
                    onUp={() => reorder(ranks, r, -1, (id) => `/rank/v1/rank-scheme/ranks/${id}`)}
                    onDown={() => reorder(ranks, r, 1, (id) => `/rank/v1/rank-scheme/ranks/${id}`)}
                  />
                  <span>
                    {r.abbreviation ? <span className="font-medium">{r.abbreviation}</span> : null}{" "}
                    {pickLabel(r.name, locale) || r.code}{" "}
                    <span className="font-mono text-xs text-slate-400">{r.code}</span>
                    {r.gradeCode ? (
                      <span className="ml-1 rounded bg-indigo-50 px-1 font-mono text-xs text-indigo-600">
                        {r.gradeCode}
                      </span>
                    ) : null}
                  </span>
                  <span className="ml-auto flex items-center gap-3">
                    <EditToggle id={r.id} editing={editing} setEditing={setEditing} />
                    <button
                      type="button"
                      className="text-xs font-medium text-red-500 hover:text-red-700"
                      onClick={() =>
                        window.confirm(`Delete rank ${r.code}?`) &&
                        run(() => mutate("DELETE", `/rank/v1/rank-scheme/rank/${r.id}`))
                      }
                    >
                      Delete
                    </button>
                  </span>
                </div>
              );
            })}

          {canAddChild ? (
            <div className="flex items-center gap-2 pt-1">
              <span className="text-xs font-medium text-slate-400">New sub-type:</span>
              <AddInline
                fields={[
                  { name: "code", placeholder: "code" },
                  { name: "name", placeholder: "name" },
                  { name: "sortOrder", placeholder: "priority", type: "number" },
                ]}
                label="Add sub-type"
                busy={busy}
                onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/types", { ...b, parentTypeId: t.id }))}
              />
            </div>
          ) : null}

          {canHoldRanks ? (
            <div className="flex items-center gap-2 pt-1">
              <span className="text-xs font-medium text-slate-400">New rank:</span>
              <AddInline
                fields={[
                  { name: "code", placeholder: "code" },
                  { name: "name", placeholder: "name" },
                  { name: "abbreviation", placeholder: "abbr" },
                  { name: "gradeCode", placeholder: "grade", options: gradeOptions },
                  { name: "sortOrder", placeholder: "priority", type: "number" },
                ]}
                label="Add rank"
                busy={busy}
                onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/ranks", { ...b, typeId: t.id }))}
              />
            </div>
          ) : null}
        </div>
      </div>
    );
  };

  const renderCategory = (cat: RankCategory, cats: RankCategory[], ci: number) => {
    const types = (cat.types ?? []).slice().sort(byOrder);
    const catPath = `/rank/v1/rank-scheme/categories/${cat.id}`;
    return (
      <div key={cat.id} className="rounded-md border border-slate-200 p-3">
        <div className="flex items-center justify-between gap-2">
          {editing === cat.id ? (
            <NodeEdit
              fields={[
                { name: "name", placeholder: "name", value: pickLabel(cat.name, locale) },
                { name: "sortOrder", placeholder: "priority", type: "number", value: numStr(cat.sortOrder) },
              ]}
              onSave={(b) => run(() => mutate("PUT", catPath, b), () => setEditing(null))}
              onCancel={() => setEditing(null)}
              busy={busy}
            />
          ) : (
            <h3 className="flex items-center gap-2 text-sm font-semibold text-slate-800">
              <PriorityChip value={cat.sortOrder} busy={busy} onSet={(n) => setPriority(catPath, n)} />
              {pickLabel(cat.name, locale) || cat.code}{" "}
              <span className="font-mono text-xs text-slate-400">{cat.code}</span>
            </h3>
          )}
          <span className="flex items-center gap-3">
            <Reorder
              busy={busy}
              upDisabled={ci === 0}
              downDisabled={ci === cats.length - 1}
              onUp={() => reorder(cats, cat, -1, (id) => `/rank/v1/rank-scheme/categories/${id}`)}
              onDown={() => reorder(cats, cat, 1, (id) => `/rank/v1/rank-scheme/categories/${id}`)}
            />
            <EditToggle id={cat.id} editing={editing} setEditing={setEditing} />
            <ActionButton
              method="DELETE"
              path={`/rank/v1/rank-scheme/category/${cat.id}`}
              label="Delete"
              confirm={`Delete category ${cat.code}? (must have no active types)`}
              tone="danger"
            />
          </span>
        </div>

        <div className="mt-3 space-y-3">
          {types.map((t, ti) => renderType(t, types, ti))}

          <div className="flex items-center gap-2 pt-1">
            <span className="text-xs font-medium text-slate-400">New type:</span>
            <AddInline
              fields={[
                { name: "code", placeholder: "code" },
                { name: "name", placeholder: "name" },
                { name: "sortOrder", placeholder: "priority", type: "number" },
              ]}
              label="Add type"
              busy={busy}
              onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/types", { ...b, categoryId: cat.id }))}
            />
          </div>
        </div>
      </div>
    );
  };

  const systems = scheme.systems.slice().sort(byOrder);

  return (
    <div className="space-y-4">
      {err ? <ErrorBox error={err} /> : null}

      {systems.map((sys: RankSystem, si) => {
        const cats = (sys.categories ?? []).slice().sort(byOrder);
        const sysPath = `/rank/v1/rank-scheme/systems/${sys.id}`;
        return (
          <div key={sys.id} className="card p-5">
            <div className="flex items-center justify-between gap-2">
              {editing === sys.id ? (
                <NodeEdit
                  fields={[
                    { name: "name", placeholder: "name", value: pickLabel(sys.name, locale) },
                    { name: "country", placeholder: "country", value: sys.country ?? "" },
                    { name: "sortOrder", placeholder: "priority", type: "number", value: numStr(sys.sortOrder) },
                  ]}
                  onSave={(b) => run(() => mutate("PUT", sysPath, b), () => setEditing(null))}
                  onCancel={() => setEditing(null)}
                  busy={busy}
                />
              ) : (
                <h2 className="flex items-center gap-2 text-base font-semibold text-slate-900">
                  <PriorityChip value={sys.sortOrder} busy={busy} onSet={(n) => setPriority(sysPath, n)} />
                  {pickLabel(sys.name, locale) || sys.code}{" "}
                  <span className="font-mono text-xs text-slate-400">{sys.code}</span>
                  {sys.country ? (
                    <span className="rounded bg-slate-100 px-1.5 text-xs text-slate-500">{sys.country}</span>
                  ) : (
                    <span className="rounded bg-slate-100 px-1.5 text-xs text-slate-500">supranational</span>
                  )}
                </h2>
              )}
              <span className="flex items-center gap-3">
                <Reorder
                  busy={busy}
                  upDisabled={si === 0}
                  downDisabled={si === systems.length - 1}
                  onUp={() => reorder(systems, sys, -1, (id) => `/rank/v1/rank-scheme/systems/${id}`)}
                  onDown={() => reorder(systems, sys, 1, (id) => `/rank/v1/rank-scheme/systems/${id}`)}
                />
                <EditToggle id={sys.id} editing={editing} setEditing={setEditing} />
                <ActionButton
                  method="DELETE"
                  path={`/rank/v1/rank-scheme/system/${sys.id}`}
                  label="Delete"
                  confirm={`Delete system ${sys.code}? (must have no active categories)`}
                  tone="danger"
                />
              </span>
            </div>

            <div className="mt-4 space-y-3">
              {cats.map((cat, ci) => renderCategory(cat, cats, ci))}

              <div className="flex items-center gap-2 pt-1">
                <span className="text-xs font-medium text-slate-400">New category:</span>
                <AddInline
                  fields={[
                    { name: "code", placeholder: "code" },
                    { name: "name", placeholder: "name" },
                    { name: "sortOrder", placeholder: "priority", type: "number" },
                  ]}
                  label="Add category"
                  busy={busy}
                  onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/categories", { ...b, systemId: sys.id }))}
                />
              </div>
            </div>
          </div>
        );
      })}

      <div className="card p-5">
        <h3 className="mb-2 text-sm font-semibold text-slate-900">Add system</h3>
        <AddInline
          fields={[
            { name: "code", placeholder: "code" },
            { name: "name", placeholder: "name" },
            { name: "country", placeholder: "country (opt.)" },
            { name: "sortOrder", placeholder: "priority", type: "number" },
          ]}
          label="Add system"
          busy={busy}
          onAdd={(b) => run(() => mutate("POST", "/rank/v1/rank-scheme/systems", b))}
        />
      </div>
    </div>
  );
}

/** Collapsible panel that imports a rank-system preset (idempotent server-side) from pasted JSON. */
export function ImportRankScheme() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [result, setResult] = useState<{ created: number; updated: number; skipped: number } | null>(null);

  const submit = () => {
    setBusy(true);
    setErr(null);
    setResult(null);
    let body: unknown;
    try {
      body = JSON.parse(text);
    } catch {
      setErr(new Error("Invalid JSON — expected { \"system\": { … } }"));
      setBusy(false);
      return;
    }
    mutate("POST", "/rank/v1/rank-scheme/import", body)
      .then((r) => {
        setResult(r as { created: number; updated: number; skipped: number });
        router.refresh();
      })
      .catch(setErr)
      .finally(() => setBusy(false));
  };

  if (!open)
    return (
      <button type="button" className="btn-ghost mt-4" onClick={() => setOpen(true)}>
        Import preset…
      </button>
    );
  return (
    <div className="card mt-4 space-y-3 p-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-slate-900">Import rank-system preset</h3>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          Close
        </button>
      </div>
      <p className="text-xs text-slate-500">
        Paste a preset of shape{" "}
        <code className="font-mono">{`{ "system": { "code", "name", "categories": [ … ] } }`}</code>. Import is
        idempotent: existing codes are updated, new ones created.
      </p>
      {err ? <ErrorBox error={err} /> : null}
      {result ? (
        <p className="text-sm text-green-700">
          Imported — created {result.created}, updated {result.updated}, skipped {result.skipped}.
        </p>
      ) : null}
      <textarea
        className="input h-48 w-full font-mono text-xs"
        placeholder={`{\n  "system": {\n    "code": "nato-generic",\n    "name": "NATO generic",\n    "categories": []\n  }\n}`}
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      <button type="button" className="btn-primary" disabled={busy || !text.trim()} onClick={submit}>
        {busy ? "Importing…" : "Import"}
      </button>
    </div>
  );
}

const numStr = (n?: number) => (n === undefined || n === null ? "" : String(n));

/** Inline number box showing the node's current priority; Enter (or blur) saves the new sortOrder. */
function PriorityChip({
  value,
  busy,
  onSet,
}: {
  value?: number;
  busy: boolean;
  onSet: (n: number) => void;
}) {
  const [v, setV] = useState(numStr(value));
  const commit = () => {
    const n = parseInt(v, 10);
    if (!Number.isNaN(n) && n !== (value ?? NaN)) onSet(n);
  };
  return (
    <input
      type="number"
      value={v}
      disabled={busy}
      onChange={(e) => setV(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          commit();
        }
      }}
      title="Priority / seniority (lower = more senior)"
      className="w-12 rounded border border-slate-300 bg-white px-1 py-0.5 text-center font-mono text-xs text-slate-600 outline-none focus:border-indigo-500"
    />
  );
}

function Reorder({
  onUp,
  onDown,
  upDisabled,
  downDisabled,
  busy,
}: {
  onUp: () => void;
  onDown: () => void;
  upDisabled: boolean;
  downDisabled: boolean;
  busy: boolean;
}) {
  return (
    <span className="inline-flex overflow-hidden rounded border border-slate-200">
      <button
        type="button"
        disabled={busy || upDisabled}
        onClick={onUp}
        title="Move up (more senior)"
        className="px-1 text-xs text-slate-500 hover:bg-slate-100 disabled:opacity-30"
      >
        ▲
      </button>
      <button
        type="button"
        disabled={busy || downDisabled}
        onClick={onDown}
        title="Move down (more junior)"
        className="border-l border-slate-200 px-1 text-xs text-slate-500 hover:bg-slate-100 disabled:opacity-30"
      >
        ▼
      </button>
    </span>
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

function FieldInput({ fld }: { fld: Field }) {
  if (fld.options)
    return (
      <select
        name={fld.name}
        className="input w-28"
        defaultValue={fld.value ?? ""}
        title={fld.placeholder}
      >
        {fld.options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    );
  return (
    <input
      name={fld.name}
      type={fld.type === "number" ? "number" : "text"}
      className={fld.type === "number" ? "input w-20" : "input w-40"}
      placeholder={fld.placeholder}
      defaultValue={fld.value}
    />
  );
}

function NodeEdit({
  fields,
  onSave,
  onCancel,
  busy,
}: {
  fields: Field[];
  onSave: (body: Record<string, string | number>) => void;
  onCancel: () => void;
  busy: boolean;
}) {
  return (
    <form
      className="flex flex-wrap items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const body: Record<string, string | number> = {};
        for (const fld of fields) {
          const raw = String(f.get(fld.name) || "").trim();
          if (!raw) continue;
          body[fld.name] = fld.type === "number" ? parseInt(raw, 10) : raw;
        }
        onSave(body);
      }}
    >
      {fields.map((fld) => (
        <FieldInput key={fld.name} fld={fld} />
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
  fields,
  label,
  busy,
  onAdd,
}: {
  fields: Field[];
  label: string;
  busy: boolean;
  onAdd: (body: Record<string, string | number>) => void;
}) {
  return (
    <form
      className="inline-flex flex-wrap items-center gap-1"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        const body: Record<string, string | number> = {};
        for (const fld of fields) {
          const raw = String(f.get(fld.name) || "").trim();
          if (!raw) continue;
          body[fld.name] = fld.type === "number" ? parseInt(raw, 10) : raw;
        }
        // require code + name (the first two text fields)
        if (body[fields[0].name] && body[fields[1].name]) {
          onAdd(body);
          form.reset();
        }
      }}
    >
      {fields.map((fld, i) =>
        fld.options ? (
          <select
            key={fld.name}
            name={fld.name}
            defaultValue=""
            title={fld.placeholder}
            className="w-24 rounded-md border border-slate-300 bg-white px-2 py-1 text-xs outline-none focus:border-indigo-500"
          >
            {fld.options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        ) : (
          <input
            key={fld.name}
            name={fld.name}
            type={fld.type === "number" ? "number" : "text"}
            required={i < 2}
            className={`${fld.type === "number" ? "w-16" : "w-28"} rounded-md border border-slate-300 bg-white px-2 py-1 text-xs outline-none focus:border-indigo-500`}
            placeholder={fld.placeholder}
          />
        ),
      )}
      <button
        className="rounded-md border border-slate-300 bg-white px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50"
        disabled={busy}
      >
        {label}
      </button>
    </form>
  );
}
