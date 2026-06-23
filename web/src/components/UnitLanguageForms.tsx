"use client";

// Unit official/working-language editor (D-Languages, M18). The link is its own sub-resource (not part
// of the Unit aggregate), so the component fetches its list and refreshes after each write.
import { useEffect, useState } from "react";
import { mutate } from "@/lib/api/client";
import { bffGet } from "@/lib/api/browser";
import { ErrorBox } from "@/components/ErrorBox";
import { LanguagePicker } from "@/components/LanguagePicker";
import { T } from "@/components/T";
import { pickLabel } from "@/lib/i18n";
import { useLocale, useTg } from "@/lib/locale";
import type { UnitLanguage } from "@/lib/api/types";

export function UnitLanguageManager({ unitId }: { unitId: string }) {
  const { locale } = useLocale();
  const tr = useTg();
  const [rows, setRows] = useState<UnitLanguage[] | null>(null);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [langId, setLangId] = useState("");
  const [pickerKey, setPickerKey] = useState(0);

  const load = () =>
    bffGet<UnitLanguage[]>(`/tenant/v1/units/${unitId}/languages`)
      .then((r) => setRows(r ?? []))
      .catch(setErr);
  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [unitId]);

  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      after?.();
      await load();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      {rows && rows.length === 0 ? <p className="text-sm text-slate-400"><T>No languages set.</T></p> : null}
      <ul className="space-y-0.5 text-sm text-slate-700">
        {(rows ?? []).map((l) => (
          <li key={l.id} className="flex items-center justify-between gap-2">
            <span>
              {pickLabel(l.name, locale) || l.languageId}
              {l.isOfficial ? tr(" · official") : tr(" · working")}
            </span>
            <button
              type="button"
              className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
              disabled={busy}
              onClick={() =>
                window.confirm("Remove this language?") &&
                run(() => mutate("DELETE", `/tenant/v1/units/${unitId}/languages/${l.languageId}`))
              }
            >
              <T>Remove</T>
            </button>
          </li>
        ))}
      </ul>
      <form
        className="mt-2 grid grid-cols-[1fr_auto_auto] items-center gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          if (!langId) return;
          const f = new FormData(ev.currentTarget);
          run(
            () =>
              mutate("PUT", `/tenant/v1/units/${unitId}/languages`, {
                languageId: langId,
                isOfficial: f.get("isOfficial") === "on",
              }),
            () => {
              setLangId("");
              setPickerKey((k) => k + 1);
            },
          );
        }}
      >
        <LanguagePicker key={pickerKey} onChange={setLangId} />
        <label className="flex items-center gap-1 text-xs text-slate-600">
          <input type="checkbox" name="isOfficial" defaultChecked /> {tr("official")}
        </label>
        <button className="btn-ghost" disabled={busy || !langId}>
          <T>Add</T>
        </button>
      </form>
    </div>
  );
}
