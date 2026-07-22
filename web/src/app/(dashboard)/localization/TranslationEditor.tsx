"use client";

import { useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";

type TransMap = { [field: string]: { [locale: string]: string } };

// Common translatable entity types (D-i18n / M45 pinax overlay) — a datalist hint, not a constraint;
// the backend accepts any registered entity type. entityId is the entity's natural key (code) or RID.
const ENTITY_TYPES = [
  "country",
  "ethnicity_type",
  "languoid",
  "writing_system",
  "religion_taxon",
  "color",
  "rank_system",
  "rank_category",
  "rank_type",
  "rank",
];

/** Editor over the i18n translation store (D-i18n): load an entity's `field → locale → text` map via
 * getTranslations and upsert it via putTranslations. Complements the locale admin above — locales say
 * *which* languages exist; this edits the actual translated labels. */
export function TranslationEditor({ locales }: { locales: { code: string; name: string }[] }) {
  const tr = useTg();
  const [entityType, setEntityType] = useState("");
  const [entityId, setEntityId] = useState("");
  const [data, setData] = useState<TransMap | null>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const [newField, setNewField] = useState("");

  const canQuery = entityType.trim() !== "" && entityId.trim() !== "";

  const load = async () => {
    if (!canQuery) return;
    setBusy(true);
    setErr(null);
    setSaved(false);
    try {
      const res = await api.localization.getTranslations(entityType.trim(), entityId.trim());
      setData((res ?? {}) as TransMap);
    } catch (e) {
      setErr(e);
      setData(null);
    } finally {
      setBusy(false);
    }
  };

  const save = async () => {
    if (!canQuery || !data) return;
    setBusy(true);
    setErr(null);
    setSaved(false);
    try {
      const res = await api.localization.putTranslations(entityType.trim(), entityId.trim(), data);
      setData((res ?? {}) as TransMap);
      setSaved(true);
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  const setCell = (field: string, locale: string, value: string) =>
    setData((prev) => ({ ...(prev ?? {}), [field]: { ...(prev?.[field] ?? {}), [locale]: value } }));

  const addField = () => {
    const f = newField.trim();
    if (!f) return;
    setData((prev) => ({ ...(prev ?? {}), [f]: prev?.[f] ?? {} }));
    setNewField("");
  };

  const fields = data ? Object.keys(data).sort() : [];

  return (
    <div className="card p-5">
      <h2 className="text-sm font-semibold text-slate-900"><T>Entity translations</T></h2>
      <p className="mb-3 mt-1 text-xs text-slate-500">
        <T>View and edit an entity's translated labels (field → locale → text). entityType is the kind (e.g. country, languoid); entityId is its code or RID.</T>
      </p>

      <div className="flex flex-wrap items-end gap-2">
        <div>
          <label className="label" htmlFor="tr-type">{tr("Entity type")}</label>
          <input
            id="tr-type"
            className="input"
            list="tr-entity-types"
            placeholder={tr("country")}
            value={entityType}
            onChange={(e) => setEntityType(e.target.value)}
          />
          <datalist id="tr-entity-types">
            {ENTITY_TYPES.map((t) => (
              <option key={t} value={t} />
            ))}
          </datalist>
        </div>
        <div className="min-w-[16rem] flex-1">
          <label className="label" htmlFor="tr-id">{tr("Entity id (code or RID)")}</label>
          <input
            id="tr-id"
            className="input"
            placeholder="UA"
            value={entityId}
            onChange={(e) => setEntityId(e.target.value)}
          />
        </div>
        <button type="button" className="btn-ghost" disabled={!canQuery || busy} onClick={load}>
          {busy ? tr("Loading…") : tr("Load")}
        </button>
      </div>

      {err ? <div className="mt-3"><ErrorBox error={err} /></div> : null}
      {saved ? <p className="mt-3 text-sm text-green-700"><T>Saved.</T></p> : null}

      {data ? (
        <div className="mt-4">
          {fields.length === 0 ? (
            <p className="text-sm text-slate-400"><T>No translations yet — add a field to start.</T></p>
          ) : (
            <div className="space-y-4">
              {fields.map((field) => (
                <div key={field}>
                  <div className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
                    {field}
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {locales.map((l) => (
                      <label key={l.code} className="flex items-center gap-2 text-sm">
                        <span className="w-10 shrink-0 font-mono text-xs text-slate-500">{l.code}</span>
                        <input
                          className="input"
                          value={data[field]?.[l.code] ?? ""}
                          onChange={(e) => setCell(field, l.code, e.target.value)}
                        />
                      </label>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}

          <div className="mt-4 flex flex-wrap items-end gap-2 border-t border-slate-100 pt-3">
            <div>
              <label className="label" htmlFor="tr-newfield">{tr("Add field")}</label>
              <input
                id="tr-newfield"
                className="input"
                placeholder={tr("name")}
                value={newField}
                onChange={(e) => setNewField(e.target.value)}
              />
            </div>
            <button type="button" className="btn-ghost" disabled={!newField.trim()} onClick={addField}>
              {tr("Add")}
            </button>
            <button type="button" className="btn-primary ml-auto" disabled={busy} onClick={save}>
              {busy ? tr("Saving…") : tr("Save translations")}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
