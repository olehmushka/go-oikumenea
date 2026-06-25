"use client";

import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { PageHeader } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { CountrySelect } from "@/components/CountrySelect";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { newSuffix, slugify } from "@/lib/code";

export default function NewPersonPage() {
  const router = useRouter();
  const tr = useTg();
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  // Live-fill: derive a code from the display name until the operator edits the code field. A stable
  // per-form suffix keeps the auto-code from churning on every keystroke.
  const suffix = useRef(newSuffix());
  const [displayName, setDisplayName] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  // Provisional stub (D-OverlayFoundation, M29): a minimal-PII node to point edges at, resolved later
  // via merge. When on, only displayName (+ attribution) is sent.
  const [provisional, setProvisional] = useState(false);
  const slug = slugify(displayName);
  const autoCode = slug ? `${slug}-${suffix.current}` : "";
  const codeValue = codeTouched ? code : autoCode;

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
      displayName: displayName.trim(),
      code: codeValue.trim() || undefined,
      given: str("given"),
      surname: str("surname"),
      birthdate: str("birthdate"),
      dateOfDeath: str("dateOfDeath"),
      sex: str("sex"),
      countryOfBirth: str("countryOfBirth"),
    };
    try {
      const p = provisional
        ? await api.person.createProvisionalPerson({
            displayName: displayName.trim(),
            confidence: str("confidence"),
          } as never)
        : await api.person.createPerson(body as never);
      router.push(`/persons/${p.id}`);
    } catch (e) {
      setErr(e);
      setBusy(false);
    }
  }

  return (
    <div className="max-w-xl">
      <PageHeader title={<T>New person</T>} description={<T>Create a directory entry. A login account is optional and attached later.</T>} />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}
      <form onSubmit={onSubmit} className="card space-y-4 p-5">
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={provisional}
            onChange={(e) => setProvisional(e.target.checked)}
          />
          <span><T>Provisional stub</T></span>
          <span className="text-xs text-zinc-500"><T>(unresolved external person — resolve later via merge)</T></span>
        </label>
        <div>
          <label className="label"><T>Display name *</T></label>
          <input
            name="displayName"
            required
            className="input"
            placeholder={tr("Ivan Petrenko")}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </div>
        {provisional ? (
          <div>
            <label className="label"><T>Confidence</T></label>
            <select name="confidence" className="input" defaultValue="possible">
              <option value="confirmed">{tr("confirmed")}</option>
              <option value="probable">{tr("probable")}</option>
              <option value="possible">{tr("possible")}</option>
            </select>
          </div>
        ) : null}
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label"><T>Given</T></label>
            <input name="given" className="input" />
          </div>
          <div>
            <label className="label"><T>Surname</T></label>
            <input name="surname" className="input" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label"><T>Code</T></label>
            <input
              name="code"
              className="input"
              placeholder={tr("auto from name")}
              value={codeValue}
              onChange={(e) => {
                setCode(e.target.value);
                setCodeTouched(true);
              }}
            />
          </div>
          <div>
            <label className="label"><T>Birthdate</T></label>
            <input name="birthdate" type="date" className="input" />
          </div>
          <div>
            <label className="label"><T>Date of death</T></label>
            <input name="dateOfDeath" type="date" className="input" />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="label"><T>Sex (ISO 5218)</T></label>
            <select name="sex" className="input" defaultValue="">
              <option value="">—</option>
              <option value="0">{tr("0 — not known")}</option>
              <option value="1">{tr("1 — male")}</option>
              <option value="2">{tr("2 — female")}</option>
              <option value="9">{tr("9 — not applicable")}</option>
            </select>
          </div>
          <div>
            <label className="label"><T>Country of birth</T></label>
            <CountrySelect name="countryOfBirth" />
          </div>
        </div>
        <div className="flex gap-2">
          <button type="submit" className="btn-primary" disabled={busy}>
            {busy ? <T>Creating…</T> : <T>Create person</T>}
          </button>
          <button type="button" className="btn-ghost" onClick={() => router.back()}>
            <T>Cancel</T>
          </button>
        </div>
      </form>
    </div>
  );
}
