"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { mutate } from "@/lib/api/client";
import { bffGet } from "@/lib/api/browser";
import { ErrorBox } from "@/components/ErrorBox";
import { EntitySelect } from "@/components/EntitySelect";
import { pickLabel } from "@/lib/i18n";
import { useLocale } from "@/lib/locale";
import { ridTail } from "@/lib/ontology/rid";
import type {
  Association,
  CallSign,
  Citizenship,
  DocumentDoc,
  Email,
  Guardianship,
  Kinship,
  LocaleMap,
  MessengerLink,
  NameVariant,
  NextOfKin,
  Partnership,
  Person,
  Phone,
  Platform,
  RelationType,
  Residence,
  SocialAccount,
  SocialAccountHandle,
  Sponsorship,
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
        // The scheme tree is system → category → type (which may nest sub-types) → rank.
        const walkType = (t: { name?: LocaleMap; code: string; children?: unknown[]; ranks?: unknown[] }, sys: string) => {
          for (const rk of (t.ranks as { id: string; name?: LocaleMap; code: string }[]) ?? [])
            out.push({ id: rk.id, label: `${sys} · ${pickLabel(rk.name, locale) || rk.code}` });
          for (const c of (t.children as typeof t[]) ?? []) walkType(c, sys);
        };
        for (const sysNode of d?.systems ?? []) {
          const sysLabel = pickLabel(sysNode.name, locale) || sysNode.code;
          for (const c of sysNode.categories ?? [])
            for (const t of c.types ?? []) walkType(t, sysLabel);
        }
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

/* ------------------------------------------------------------------ social & messenger (M13) */

const pf = (code: string, platforms: Platform[], locale: string) =>
  pickLabel(platforms.find((p) => p.code === code)?.name, locale) || code;

/** Social accounts: platform handle + attribution (source/confidence) + verification + rename history. */
export function SocialAccountManager({
  personId,
  accounts,
  platforms,
}: {
  personId: string;
  accounts?: SocialAccount[];
  platforms: Platform[];
}) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title="Social accounts" err={err}>
      {accounts && accounts.length ? (
        <ul className="mt-1 space-y-1 text-sm text-slate-700">
          {accounts.map((a) => (
            <li key={a.id} className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="truncate">
                  <span className="font-medium">@{a.handle}</span>
                  {a.isPrimary ? " ★" : ""}{" "}
                  <span className="text-slate-400">· {pf(a.platformCode, platforms, locale)}</span>
                </div>
                <div className="flex flex-wrap items-center gap-1 text-xs text-slate-500">
                  <span>{a.source}</span>
                  <span>· {a.confidence}</span>
                  {a.platformVerified ? <span className="text-green-600">· ✓ platform</span> : null}
                  {a.verifiedByOperatorAt ? <span className="text-green-600">· ✓ operator</span> : null}
                  <HandleHistory personId={personId} accountId={a.id} />
                </div>
              </div>
              <RowDelete
                path={`/person/v1/persons/${personId}/social-accounts/${a.id}`}
                confirm="Remove this social account?"
              />
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-1 text-sm text-slate-400">—</p>
      )}
      <form
        className="mt-2 grid grid-cols-[8rem_1fr_8rem_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/social-accounts`, {
                platformCode: s(f, "platformCode"),
                handle: s(f, "handle"),
                source: s(f, "source"),
                confidence: s(f, "confidence"),
                displayName: s(f, "displayName"),
                profileUrl: s(f, "profileUrl"),
                platformVerified: f.get("platformVerified") === "on",
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <select name="platformCode" required className="input" defaultValue="">
          <option value="" disabled>
            platform…
          </option>
          {platforms.map((p) => (
            <option key={p.code} value={p.code}>
              {pickLabel(p.name, locale) || p.code}
            </option>
          ))}
        </select>
        <input name="handle" required className="input" placeholder="handle" />
        <select name="source" required className="input" defaultValue="self_declared">
          <option value="self_declared">self-declared</option>
          <option value="operator_verified">operator-verified</option>
          <option value="imported">imported</option>
        </select>
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
        <label className="col-span-4 flex items-center gap-3 text-xs text-slate-500">
          <span className="inline-flex items-center gap-1">
            <input type="checkbox" name="platformVerified" /> platform-verified
          </span>
          <span className="inline-flex items-center gap-1">
            <input type="checkbox" name="isPrimary" /> primary
          </span>
        </label>
      </form>
    </ChannelBlock>
  );
}

/** Inline disclosure that lazy-loads a social account's @handle rename history. */
function HandleHistory({ personId, accountId }: { personId: string; accountId: string }) {
  const [open, setOpen] = useState(false);
  const [rows, setRows] = useState<SocialAccountHandle[] | null>(null);
  const toggle = () => {
    const next = !open;
    setOpen(next);
    if (next && rows === null)
      bffGet<SocialAccountHandle[]>(`/person/v1/persons/${personId}/social-accounts/${accountId}/handles`)
        .then(setRows)
        .catch(() => setRows([]));
  };
  return (
    <>
      <button type="button" className="text-indigo-600 hover:underline" onClick={toggle}>
        · {open ? "hide" : "history"}
      </button>
      {open ? (
        <ul className="mt-1 w-full pl-3 text-xs text-slate-500">
          {rows === null ? (
            <li>loading…</li>
          ) : rows.length === 0 ? (
            <li>no rename history</li>
          ) : (
            rows.map((h) => (
              <li key={h.id}>
                @{h.handle} <span className="text-slate-400">{h.validFrom?.slice(0, 10)}</span>
                {h.validTo ? ` → ${h.validTo.slice(0, 10)}` : " (current)"}
              </li>
            ))
          )}
        </ul>
      ) : null}
    </>
  );
}

/** Messenger reachability: a platform link over an existing phone XOR email. */
export function MessengerLinkManager({
  personId,
  links,
  platforms,
  emails,
  phones,
}: {
  personId: string;
  links?: MessengerLink[];
  platforms: Platform[];
  emails?: Email[];
  phones?: Phone[];
}) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  const messengers = platforms.filter((p) => p.category === "messenger");
  const channelLabel = (l: MessengerLink) => {
    if (l.phoneId) return phones?.find((p) => p.id === l.phoneId)?.number ?? ridTail(l.phoneId);
    if (l.emailId) return emails?.find((e) => e.id === l.emailId)?.address ?? ridTail(l.emailId);
    return "—";
  };
  return (
    <ChannelBlock title="Messenger links" err={err}>
      <ItemList
        items={links}
        render={(l) =>
          `${pf(l.platformCode, platforms, locale)} → ${channelLabel(l)}${l.isPrimary ? " ★" : ""}${
            l.verifiedAt ? " ✓" : ""
          }`
        }
        del={(l) => `/person/v1/persons/${personId}/messenger-links/${l.id}`}
        delConfirm="Remove this messenger link?"
      />
      <form
        className="mt-2 grid grid-cols-[8rem_1fr_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          // The channel <select> encodes the kind: "phone:<id>" or "email:<id>" (XOR enforced by UI).
          const [kind, id] = String(f.get("channel") || "").split(":");
          run(
            () =>
              mutate("PUT", `/person/v1/persons/${personId}/messenger-links`, {
                platformCode: s(f, "platformCode"),
                phoneId: kind === "phone" ? id : undefined,
                emailId: kind === "email" ? id : undefined,
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <select name="platformCode" required className="input" defaultValue="">
          <option value="" disabled>
            messenger…
          </option>
          {messengers.map((p) => (
            <option key={p.code} value={p.code}>
              {pickLabel(p.name, locale) || p.code}
            </option>
          ))}
        </select>
        <select name="channel" required className="input" defaultValue="">
          <option value="" disabled>
            phone or email…
          </option>
          {phones && phones.length ? (
            <optgroup label="Phones">
              {phones.map((p) => (
                <option key={p.id} value={`phone:${p.id}`}>
                  {p.number}
                </option>
              ))}
            </optgroup>
          ) : null}
          {emails && emails.length ? (
            <optgroup label="Emails">
              {emails.map((e) => (
                <option key={e.id} value={`email:${e.id}`}>
                  {e.address}
                </option>
              ))}
            </optgroup>
          ) : null}
        </select>
        <button className="btn-ghost" disabled={busy}>
          Add
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ relationships (M14) */

// One row per relation family, sharing the ChannelBlock + EntitySelect + status pattern. Each family
// upserts to its own collection; all share the DELETE .../relationships/{id} sink. The counterpart
// person is picked via EntitySelect; relation-code <select>s are fed by category-filtered RelationTypes.
type RelRow = { id: string; counterpart: string; sub: string; tone?: "green" | "amber" | "slate" };

export function RelationshipManager({
  personId,
  partnerships,
  kinships,
  guardianships,
  sponsorships,
  nextOfKin,
  associations,
  relationTypes,
}: {
  personId: string;
  partnerships?: Partnership[];
  kinships?: Kinship[];
  guardianships?: Guardianship[];
  sponsorships?: Sponsorship[];
  nextOfKin?: NextOfKin[];
  associations?: Association[];
  relationTypes: RelationType[];
}) {
  const other = (a: string, b: string) => (a === personId ? b : a);
  const relTone = (st?: string) =>
    (st ?? "").toLowerCase() === "active" || (st ?? "").toLowerCase() === "married"
      ? "green"
      : ["ended", "withdrawn", "disestablished", "divorced", "dissolved", "annulled"].includes(
            (st ?? "").toLowerCase(),
          )
        ? "slate"
        : "amber";
  return (
    <div className="space-y-1">
      <RelFamily
        title="Partnerships"
        personId={personId}
        rows={(partnerships ?? []).map((r) => ({
          id: r.id,
          counterpart: other(r.personIdA, r.personIdB),
          sub: [r.status, r.effectiveFrom].filter(Boolean).join(" · "),
          tone: relTone(r.status),
        }))}
        upsertPath="/partnerships"
        counterpartField="partnerId"
        extra={
          <>
            <select name="status" required className="input" defaultValue="married">
              {["engaged", "married", "divorced", "widowed", "annulled", "dissolved"].map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </select>
            <EffectiveDates />
          </>
        }
      />
      <RelFamily
        title="Kinships (parent → child)"
        personId={personId}
        rows={(kinships ?? []).map((r) => ({
          id: r.id,
          counterpart: `${r.parentId === personId ? "child" : "parent"}: ${other(r.parentId, r.childId)}`,
          sub: r.status,
          tone: relTone(r.status),
        }))}
        upsertPath="/kinships"
        counterpartField="counterpartId"
        extra={
          <select name="role" required className="input" defaultValue="child">
            <option value="child">they are my child</option>
            <option value="parent">they are my parent</option>
          </select>
        }
      />
      <RelFamily
        title="Guardianships"
        personId={personId}
        rows={(guardianships ?? []).map((r) => ({
          id: r.id,
          counterpart: `${r.guardianId === personId ? "ward" : "guardian"}: ${other(r.guardianId, r.wardId)}`,
          sub: [r.status, r.relationCode].filter(Boolean).join(" · "),
          tone: relTone(r.status),
        }))}
        upsertPath="/guardianships"
        counterpartField="counterpartId"
        extra={
          <>
            <select name="role" required className="input" defaultValue="ward">
              <option value="ward">they are my ward</option>
              <option value="guardian">they are my guardian</option>
            </select>
            <EffectiveDates />
          </>
        }
      />
      <RelFamily
        title="Sponsorships"
        personId={personId}
        rows={(sponsorships ?? []).map((r) => ({
          id: r.id,
          counterpart: `${r.sponsorId === personId ? "sponsored" : "sponsor"}: ${other(r.sponsorId, r.sponsoredId)}`,
          sub: [r.status, r.relationCode].filter(Boolean).join(" · "),
          tone: relTone(r.status),
        }))}
        upsertPath="/sponsorships"
        counterpartField="counterpartId"
        extra={
          <>
            <select name="role" required className="input" defaultValue="sponsored">
              <option value="sponsored">they are sponsored by me</option>
              <option value="sponsor">they sponsor me</option>
            </select>
            <RelationCodeSelect types={relationTypes} category="sponsorship" required />
            <EffectiveDates />
          </>
        }
      />
      <RelFamily
        title="Next of kin"
        personId={personId}
        rows={(nextOfKin ?? []).map((r) => ({
          id: r.id,
          counterpart: other(r.subjectId, r.contactId),
          sub: [`#${r.priority}`, r.status, r.relationCode].filter(Boolean).join(" · "),
          tone: relTone(r.status),
        }))}
        upsertPath="/next-of-kin"
        counterpartField="contactId"
        extra={
          <>
            <input name="priority" type="number" min={1} className="input w-16" placeholder="#" defaultValue={1} />
            <RelationCodeSelect types={relationTypes} category="next_of_kin" />
          </>
        }
      />
      <RelFamily
        title="Associations"
        personId={personId}
        rows={(associations ?? []).map((r) => ({
          id: r.id,
          counterpart: other(r.personIdA, r.personIdB),
          sub: [r.kind, r.status, r.relationCode].filter(Boolean).join(" · "),
          tone: r.kind === "no_contact" ? "slate" : relTone(r.status),
        }))}
        upsertPath="/associations"
        counterpartField="counterpartId"
        extra={
          <>
            <select name="kind" required className="input" defaultValue="associate">
              <option value="associate">associate</option>
              <option value="coi">conflict of interest</option>
              <option value="no_contact">no contact</option>
            </select>
            <RelationCodeSelect types={relationTypes} category="association" />
          </>
        }
      />
    </div>
  );
}

/** Optional effective-from/to date pair for time-bounded relationships. */
function EffectiveDates() {
  return (
    <span className="inline-flex items-center gap-1">
      <input name="effectiveFrom" type="date" className="input w-36" title="effective from" />
      <span className="text-xs text-slate-400">→</span>
      <input name="effectiveTo" type="date" className="input w-36" title="effective to" />
    </span>
  );
}

function RelationCodeSelect({
  types,
  category,
  required = false,
}: {
  types: RelationType[];
  category: string;
  required?: boolean;
}) {
  const { locale } = useLocale();
  const opts = types.filter((t) => t.category === category);
  return (
    <select name="relationCode" required={required} className="input" defaultValue="">
      <option value="">{required ? "relation…" : "relation (optional)"}</option>
      {opts.map((t) => (
        <option key={t.code} value={t.code}>
          {pickLabel(t.name, locale) || t.code}
        </option>
      ))}
    </select>
  );
}

function RelFamily({
  title,
  personId,
  rows,
  upsertPath,
  counterpartField,
  extra,
}: {
  title: string;
  personId: string;
  rows: RelRow[];
  upsertPath: string;
  counterpartField: "partnerId" | "contactId" | "counterpartId";
  extra: React.ReactNode;
}) {
  const { busy, err, run } = useRun();
  return (
    <ChannelBlock title={title} err={err}>
      {rows.length ? (
        <ul className="mt-1 space-y-0.5 text-sm text-slate-700">
          {rows.map((r) => (
            <li key={r.id} className="flex items-center justify-between gap-2">
              <span className="flex items-center gap-2">
                <span className="font-mono text-xs">{r.counterpart.replace(/(urn:[^ ]+)/, (m) => ridTail(m))}</span>
                <span className="rounded-full bg-slate-100 px-1.5 text-xs text-slate-500">{r.sub}</span>
              </span>
              <RowDelete
                path={`/person/v1/persons/${personId}/relationships/${r.id}`}
                confirm="Remove this relationship?"
              />
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-1 text-sm text-slate-400">—</p>
      )}
      <form
        className="mt-2 flex flex-wrap items-end gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          const counterpart = String(f.get(counterpartField) || "").trim();
          if (!counterpart) return;
          const body: Record<string, unknown> = { [counterpartField]: counterpart };
          for (const k of ["status", "role", "kind", "relationCode", "effectiveFrom", "effectiveTo"]) {
            const v = s(f, k);
            if (v) body[k] = v;
          }
          const prio = s(f, "priority");
          if (prio) body.priority = parseInt(prio, 10);
          run(() => mutate("PUT", `/person/v1/persons/${personId}${upsertPath}`, body), () => form.reset());
        }}
      >
        <div className="min-w-[14rem] flex-1">
          <EntitySelect kind="person" name={counterpartField} required placeholder="counterpart person…" />
        </div>
        {extra}
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
