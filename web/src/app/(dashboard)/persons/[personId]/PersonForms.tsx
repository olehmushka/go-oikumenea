"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { ErrorBox } from "@/components/ErrorBox";
import { pickLabel } from "@/lib/i18n";
import { useLocale } from "@/lib/locale";
import type {
  CallSign,
  Citizenship,
  DocumentDoc,
  Email,
  LocaleMap,
  NameVariant,
  Person,
  Phone,
  Residence,
} from "@/lib/api/types";

type CodeRow = { id: string; schemeCode?: string; status?: string };
type ContactType = { code: string; name?: LocaleMap };
type Catalog = { id: string; code: string; name?: LocaleMap };

// Shared submit helper: runs a mutation, refreshes the route, captures the error.
function useRun() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      after?.();
      router.refresh();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };
  return { busy, err, run, setErr };
}

function s(f: FormData, k: string): string | undefined {
  const v = String(f.get(k) || "").trim();
  return v || undefined;
}

function RowDelete({ path, confirm }: { path: string; confirm: string }) {
  const { busy, run } = useRun();
  return (
    <button
      type="button"
      className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
      disabled={busy}
      onClick={() => window.confirm(confirm) && run(() => mutate("DELETE", path))}
    >
      Remove
    </button>
  );
}

/* ------------------------------------------------------------------ core identity */

export function EditPerson({ person }: { person: Person }) {
  const { busy, err, run } = useRun();
  const [open, setOpen] = useState(false);
  if (!open) {
    return (
      <button type="button" className="btn-ghost" onClick={() => setOpen(true)}>
        Edit
      </button>
    );
  }
  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        const f = new FormData(e.currentTarget);
        run(
          () =>
            mutate("PUT", `/person/v1/persons/${person.id}`, {
              displayName: s(f, "displayName"),
              given: s(f, "given"),
              surname: s(f, "surname"),
              birthdate: s(f, "birthdate"),
              sex: s(f, "sex"),
              countryOfBirth: s(f, "countryOfBirth"),
            }),
          () => setOpen(false),
        );
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900">Edit person</h3>
      {err ? <ErrorBox error={err} /> : null}
      <div>
        <label className="label">Display name</label>
        <input name="displayName" className="input" defaultValue={person.displayName} />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label">Given</label>
          <input name="given" className="input" defaultValue={person.given} />
        </div>
        <div>
          <label className="label">Surname</label>
          <input name="surname" className="input" defaultValue={person.surname} />
        </div>
      </div>
      <div className="grid grid-cols-3 gap-3">
        <div>
          <label className="label">Birthdate</label>
          <input name="birthdate" type="date" className="input" defaultValue={person.birthdate} />
        </div>
        <div>
          <label className="label">Sex (ISO 5218)</label>
          <select name="sex" className="input" defaultValue={person.sex ?? ""}>
            <option value="">—</option>
            <option value="0">0 — not known</option>
            <option value="1">1 — male</option>
            <option value="2">2 — female</option>
            <option value="9">9 — not applicable</option>
          </select>
        </div>
        <div>
          <label className="label">Country of birth</label>
          <input name="countryOfBirth" className="input" defaultValue={person.countryOfBirth} />
        </div>
      </div>
      <div className="flex gap-2">
        <button type="submit" className="btn-primary" disabled={busy}>
          {busy ? "Saving…" : "Save"}
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          Cancel
        </button>
      </div>
    </form>
  );
}

/** Lifecycle: deactivate / reactivate / purge, depending on current status. */
export function PersonLifecycle({ person }: { person: Person }) {
  const { busy, err, run } = useRun();
  const status = (person.status ?? "").toUpperCase();
  const active = status === "ACTIVE";
  return (
    <div className="flex flex-wrap items-center gap-2">
      {active ? (
        <button
          type="button"
          className="btn-ghost"
          disabled={busy}
          onClick={() =>
            window.confirm("Deactivate this person? (reversible, opens a purge grace window)") &&
            run(() => mutate("POST", `/person/v1/persons/${person.id}/deactivate`, {}))
          }
        >
          Deactivate
        </button>
      ) : (
        <>
          <button
            type="button"
            className="btn-ghost"
            disabled={busy}
            onClick={() => run(() => mutate("POST", `/person/v1/persons/${person.id}/reactivate`, {}))}
          >
            Reactivate
          </button>
          <button
            type="button"
            className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
            disabled={busy}
            onClick={() =>
              window.confirm(
                "Purge this person? This irreversibly erases PII (only allowed after the grace window).",
              ) && run(() => mutate("POST", `/person/v1/persons/${person.id}/purge`, {}))
            }
          >
            Purge
          </button>
        </>
      )}
      {err ? <ErrorBox error={err} /> : null}
    </div>
  );
}

/** Set or clear the person's single rank (flattened from the rank scheme). */
export function SetRank({ personId, currentRankId }: { personId: string; currentRankId?: string }) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  const [ranks, setRanks] = useState<{ id: string; label: string }[]>([]);
  const [value, setValue] = useState(currentRankId ?? "");

  useEffect(() => {
    fetch("/api/oikumenea/rank/v1/rank-scheme")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        const out: { id: string; label: string }[] = [];
        for (const c of d?.categories ?? [])
          for (const t of c.types ?? [])
            for (const rk of t.ranks ?? [])
              out.push({ id: rk.id, label: `${pickLabel(rk.name, locale) || rk.code}` });
        setRanks(out);
      })
      .catch(() => {});
  }, [locale]);

  return (
    <div className="flex items-end gap-2">
      <div className="flex-1">
        <label className="label">Rank</label>
        <select className="input" value={value} onChange={(e) => setValue(e.target.value)}>
          <option value="">— none —</option>
          {ranks.map((r) => (
            <option key={r.id} value={r.id}>
              {r.label}
            </option>
          ))}
        </select>
      </div>
      <button
        type="button"
        className="btn-primary"
        disabled={busy}
        onClick={() =>
          run(() =>
            mutate("PUT", `/person/v1/persons/${personId}/rank`, { rankId: value || undefined }),
          )
        }
      >
        {busy ? "…" : "Set rank"}
      </button>
      {err ? <ErrorBox error={err} /> : null}
    </div>
  );
}

/* ------------------------------------------------------------------ contact channels */

export function EmailManager({
  personId,
  emails,
  types,
}: {
  personId: string;
  emails?: Email[];
  types: ContactType[];
}) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Emails" err={err}>
      <ItemList
        items={emails}
        render={(e) => `${e.address}${e.isPrimary ? " ★" : ""} · ${e.typeCode ?? ""}`}
        del={(e) => `/person/v1/persons/${personId}/emails/${e.id}`}
        delConfirm="Remove this email?"
      />
      <form
        className="mt-2 grid grid-cols-[1fr_8rem_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/emails`, {
                address: s(f, "address"),
                typeCode: s(f, "typeCode"),
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="address" required className="input" placeholder="email@example.com" />
        <select name="typeCode" required className="input">
          {types.map((t) => (
            <option key={t.code} value={t.code}>
              {pickLabel(t.name, locale) || t.code}
            </option>
          ))}
        </select>
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

export function PhoneManager({
  personId,
  phones,
  types,
}: {
  personId: string;
  phones?: Phone[];
  types: ContactType[];
}) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Phones" err={err}>
      <ItemList
        items={phones}
        render={(p) => `${p.number}${p.isPrimary ? " ★" : ""} · ${p.typeCode ?? ""}`}
        del={(p) => `/person/v1/persons/${personId}/phones/${p.id}`}
        delConfirm="Remove this phone?"
      />
      <form
        className="mt-2 grid grid-cols-[1fr_8rem_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/phones`, {
                number: s(f, "number"),
                typeCode: s(f, "typeCode"),
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="number" required className="input" placeholder="+380…" />
        <select name="typeCode" required className="input">
          {types.map((t) => (
            <option key={t.code} value={t.code}>
              {pickLabel(t.name, locale) || t.code}
            </option>
          ))}
        </select>
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

export function CallSignManager({ personId, callSigns }: { personId: string; callSigns?: CallSign[] }) {
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Call signs" err={err}>
      <ItemList
        items={callSigns}
        render={(c) => `${c.callSign}${c.isPrimary ? " ★" : ""}`}
        del={(c) => `/person/v1/persons/${personId}/call-signs/${c.id}`}
        delConfirm="Remove this call sign?"
      />
      <form
        className="mt-2 flex gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/call-signs`, {
                callSign: s(f, "callSign"),
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="callSign" required className="input" placeholder="call sign" />
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ citizenship / residence */

export function CitizenshipManager({
  personId,
  citizenships,
}: {
  personId: string;
  citizenships?: Citizenship[];
}) {
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Citizenships" err={err}>
      <ItemList
        items={citizenships}
        render={(c) => `${c.country}${c.isPrimary ? " (primary)" : ""}${c.basis ? ` · ${c.basis}` : ""}`}
        del={(c) => `/person/v1/persons/${personId}/citizenships/${c.country}`}
        delConfirm="Remove this citizenship?"
      />
      <form
        className="mt-2 grid grid-cols-[6rem_1fr_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/citizenships`, {
                country: s(f, "country"),
                basis: s(f, "basis"),
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="country" required className="input" placeholder="UKR" />
        <select name="basis" className="input" defaultValue="">
          <option value="">basis…</option>
          <option value="birth">birth</option>
          <option value="descent">descent</option>
          <option value="naturalization">naturalization</option>
          <option value="other">other</option>
        </select>
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

export function ResidenceManager({
  personId,
  residences,
}: {
  personId: string;
  residences?: Residence[];
}) {
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Residences" err={err}>
      <ItemList
        items={residences}
        render={(r) => [r.country, r.region].filter(Boolean).join(" / ")}
        del={(r) => `/person/v1/persons/${personId}/residences/${r.id}`}
        delConfirm="Remove this residence?"
      />
      <form
        className="mt-2 grid grid-cols-[5rem_1fr_8rem_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/residences`, {
                country: s(f, "country"),
                region: s(f, "region"),
                validFrom: s(f, "validFrom") ?? new Date().toISOString().slice(0, 10),
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="country" required className="input" placeholder="UKR" />
        <input name="region" className="input" placeholder="region (optional)" />
        <input name="validFrom" type="date" className="input" />
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ name variants */

export function NameVariantManager({
  personId,
  variants,
}: {
  personId: string;
  variants?: NameVariant[];
}) {
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Name variants" err={err}>
      <ItemList
        items={variants}
        render={(n) => `${n.locale}: ${n.displayName ?? `${n.given ?? ""} ${n.surname ?? ""}`}${n.isPrimary ? " ★" : ""}`}
        del={(n) => `/person/v1/persons/${personId}/name-variants/${n.locale}`}
        delConfirm="Remove this name variant?"
      />
      <form
        className="mt-2 grid grid-cols-[5rem_1fr_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/name-variants`, {
                locale: s(f, "locale"),
                displayName: s(f, "displayName"),
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="locale" required className="input" placeholder="ukr" />
        <input name="displayName" required className="input" placeholder="Іван Петренко" />
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ documents / personal codes */

export function DocumentManager({
  personId,
  documents,
  types,
}: {
  personId: string;
  documents?: DocumentDoc[];
  types: Catalog[];
}) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Documents" err={err}>
      <ItemList
        items={documents}
        render={(d) => `${d.number ?? d.id.slice(-8)} · ${d.issuingCountry ?? ""} · ${d.status ?? ""}`}
        del={(d) => `/document/v1/documents/${d.id}`}
        delConfirm="Delete this document?"
      />
      <form
        className="mt-2 grid grid-cols-[1fr_1fr_6rem_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("POST", `/document/v1/persons/${personId}/documents`, {
                typeId: s(f, "typeId"),
                number: s(f, "number"),
                issuingCountry: s(f, "issuingCountry"),
              }),
            () => form.reset(),
          );
        }}
      >
        <select name="typeId" required className="input" defaultValue="">
          <option value="" disabled>
            type…
          </option>
          {types.map((t) => (
            <option key={t.id} value={t.id}>
              {pickLabel(t.name, locale) || t.code}
            </option>
          ))}
        </select>
        <input name="number" className="input" placeholder="number" />
        <input name="issuingCountry" className="input" placeholder="UKR" />
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

export function PersonalCodeManager({
  personId,
  codes,
  schemes,
}: {
  personId: string;
  codes?: CodeRow[];
  schemes: { code: string; name?: LocaleMap }[];
}) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Personal codes" err={err}>
      <ItemList
        items={codes}
        render={(c) => `${c.schemeCode ?? "?"} · ${c.status ?? "active"} (value encrypted)`}
        del={(c) => `/document/v1/personal-codes/${c.id}`}
        delConfirm="Delete this personal code?"
      />
      <form
        className="mt-2 grid grid-cols-[1fr_1fr_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("POST", `/document/v1/persons/${personId}/personal-codes`, {
                schemeCode: s(f, "schemeCode"),
                value: s(f, "value"),
              }),
            () => form.reset(),
          );
        }}
      >
        <select name="schemeCode" required className="input" defaultValue="">
          <option value="" disabled>
            scheme…
          </option>
          {schemes.map((sch) => (
            <option key={sch.code} value={sch.code}>
              {pickLabel(sch.name, locale) || sch.code}
            </option>
          ))}
        </select>
        <input name="value" required className="input" placeholder="identifier value" />
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ small shared UI */

function ChannelBlock({
  title,
  err,
  children,
}: {
  title: string;
  err: unknown;
  children: React.ReactNode;
}) {
  return (
    <div className="mt-3">
      <div className="text-xs font-medium uppercase tracking-wide text-slate-400">{title}</div>
      {err ? <div className="mt-1"><ErrorBox error={err} /></div> : null}
      {children}
    </div>
  );
}

function ItemList<T extends { id?: string }>({
  items,
  render,
  del,
  delConfirm,
}: {
  items?: T[];
  render: (it: T) => string;
  del: (it: T) => string;
  delConfirm: string;
}) {
  if (!items || items.length === 0)
    return <p className="mt-1 text-sm text-slate-400">—</p>;
  return (
    <ul className="mt-1 space-y-0.5 text-sm text-slate-700">
      {items.map((it, i) => (
        <li key={it.id ?? i} className="flex items-center justify-between gap-2">
          <span>{render(it)}</span>
          <RowDelete path={del(it)} confirm={delConfirm} />
        </li>
      ))}
    </ul>
  );
}
