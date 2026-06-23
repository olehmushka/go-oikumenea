"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";

export function AddLocale() {
  const router = useRouter();
  const tr = useTg();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        const form = e.currentTarget;
        setBusy(true);
        setErr(null);
        (async () => {
          try {
            await api.localization.addLocale({
              code: String(f.get("code") || "").trim(),
              name: String(f.get("name") || "").trim(),
              enabled: true,
            });
            form.reset();
            router.refresh();
          } catch (e) {
            setErr(e);
          } finally {
            setBusy(false);
          }
        })();
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900"><T>Add locale</T></h3>
      <p className="text-xs text-slate-500"><T>ISO 639-3 code, e.g.</T> <code>pol</code>, <code>deu</code>.</p>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input name="code" required maxLength={3} className="input" placeholder={tr("code")} />
        <input name="name" required className="input" placeholder={tr("display name")} />
      </div>
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? <T>Adding…</T> : <T>Add locale</T>}
      </button>
    </form>
  );
}

export function ToggleLocale({ code, enabled }: { code: string; enabled?: boolean }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  return (
    <button
      className="text-xs font-medium text-indigo-600 hover:underline disabled:opacity-50"
      disabled={busy}
      onClick={async () => {
        setBusy(true);
        try {
          await api.localization.updateLocale(code, { enabled: !enabled });
          router.refresh();
        } finally {
          setBusy(false);
        }
      }}
    >
      {busy ? "…" : enabled ? <T>Disable</T> : <T>Enable</T>}
    </button>
  );
}
