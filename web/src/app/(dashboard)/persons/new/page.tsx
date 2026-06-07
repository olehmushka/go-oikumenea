"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { mutate } from "@/lib/api/client";
import { PageHeader } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";

export default function NewPersonPage() {
  const router = useRouter();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    const f = new FormData(e.currentTarget);
    const str = (k: string) => {
      const v = String(f.get(k) || "").trim();
      return v || undefined;
    };
    const body = {
      displayName: String(f.get("displayName") || "").trim(),
      code: str("code"),
      given: str("given"),
      surname: str("surname"),
      birthdate: str("birthdate"),
      sex: str("sex"),
      countryOfBirth: str("countryOfBirth"),
    };
    try {
      const p = await mutate<{ id: string }>("POST", "/person/v1/persons", body);
      router.push(`/persons/${p.id}`);
    } catch (e) {
      setErr(e);
      setBusy(false);
    }
  }

  return (
    <div className="max-w-xl">
      <PageHeader title="New person" description="Create a directory entry. A login account is optional and attached later." />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="card space-y-4 p-5">
        <div>
          <label className="label">Display name *</label>
          <input name="displayName" required className="input" placeholder="Ivan Petrenko" />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label">Given</label>
            <input name="given" className="input" />
          </div>
          <div>
            <label className="label">Surname</label>
            <input name="surname" className="input" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label">Code</label>
            <input name="code" className="input" placeholder="external ref" />
          </div>
          <div>
            <label className="label">Birthdate</label>
            <input name="birthdate" type="date" className="input" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label">Sex (ISO 5218)</label>
            <select name="sex" className="input" defaultValue="">
              <option value="">—</option>
              <option value="0">0 — not known</option>
              <option value="1">1 — male</option>
              <option value="2">2 — female</option>
              <option value="9">9 — not applicable</option>
            </select>
          </div>
          <div>
            <label className="label">Country of birth (ISO-3166)</label>
            <input name="countryOfBirth" className="input" placeholder="UKR" />
          </div>
        </div>
        <div className="flex gap-2">
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? "Creating…" : "Create person"}
          </button>
          <button type="button" className="btn-ghost" onClick={() => router.back()}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
