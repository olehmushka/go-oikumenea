"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { useCatalog } from "@/lib/catalog";
import { ErrorBox } from "@/components/ErrorBox";
import { EntitySelect } from "@/components/EntitySelect";
import { CountrySelect, useCountryMap } from "@/components/CountrySelect";
import { LanguagePicker } from "@/components/LanguagePicker";
import { ColorPicker } from "@/components/ColorPicker";
import { EthnicityPicker } from "@/components/EthnicityPicker";
import { ReligionTaxonPicker } from "@/components/ReligionTaxonPicker";
import { PersonLink } from "@/components/PositionForms";
import { SearchSelect } from "@/components/SearchSelect";
import { T } from "@/components/T";
import { pickLabel } from "@/lib/i18n";
import { useLocale, useTg } from "@/lib/locale";
import { ridTail } from "@/lib/ontology/rid";
import type {
  Affiliation,
  AffiliationType,
  Association,
  CallSign,
  Citizenship,
  ClergyCredential,
  ClergyGrade,
  DistinguishingMark,
  DocumentDoc,
  Email,
  Ethnicity,
  Guardianship,
  Kinship,
  LocaleMap,
  MessengerLink,
  Address,
  NameVariant,
  NextOfKin,
  Partnership,
  Person,
  PersonLanguage,
  PersonRank,
  PhysicalDescription,
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
      onClick={() => window.confirm(confirm) && run(() => api.request("DELETE", path))}
    >
      <T>Remove</T>
    </button>
  );
}

/* ------------------------------------------------------------------ core identity */

export function EditPerson({ person }: { person: Person }) {
  const { busy, err, run } = useRun();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  if (!open) {
    return (
      <button type="button" className="btn-ghost" onClick={() => setOpen(true)}>
        <T>Edit</T>
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
            api.person.updatePerson(person.id, {
              displayName: s(f, "displayName"),
              given: s(f, "given"),
              surname: s(f, "surname"),
              birthdate: s(f, "birthdate"),
              dateOfDeath: s(f, "dateOfDeath"),
              sex: s(f, "sex"),
              countryOfBirth: s(f, "countryOfBirth"),
            }),
          () => setOpen(false),
        );
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900"><T>Edit person</T></h3>
      {err ? <ErrorBox error={err} /> : null}
      <div>
        <label className="label"><T>Display name</T></label>
        <input name="displayName" className="input" defaultValue={person.displayName} />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label"><T>Given</T></label>
          <input name="given" className="input" defaultValue={person.given ?? ""} />
        </div>
        <div>
          <label className="label"><T>Surname</T></label>
          <input name="surname" className="input" defaultValue={person.surname ?? ""} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label"><T>Birthdate</T></label>
          <input name="birthdate" type="date" className="input" defaultValue={person.birthdate ?? ""} />
        </div>
        <div>
          <label className="label"><T>Date of death</T></label>
          <input name="dateOfDeath" type="date" className="input" defaultValue={person.dateOfDeath ?? ""} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className="label"><T>Sex (ISO 5218)</T></label>
          <select name="sex" className="input" defaultValue={person.sex ?? ""}>
            <option value="">—</option>
            <option value="0">{tr("0 — not known")}</option>
            <option value="1">{tr("1 — male")}</option>
            <option value="2">{tr("2 — female")}</option>
            <option value="9">{tr("9 — not applicable")}</option>
          </select>
        </div>
        <div>
          <label className="label"><T>Country of birth</T></label>
          <CountrySelect name="countryOfBirth" defaultValue={person.countryOfBirth ?? undefined} />
        </div>
      </div>
      <div className="flex gap-2">
        <button type="submit" className="btn-primary" disabled={busy}>
          {busy ? <T>Saving…</T> : <T>Save</T>}
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          <T>Cancel</T>
        </button>
      </div>
    </form>
  );
}

/** Lifecycle: deactivate / reactivate / purge, depending on current status. */
// MergeProvisional resolves a provisional stub into a canonical person (D-OverlayFoundation, M29):
// it re-homes the stub's edges (and every other module's references) onto the canonical person, then
// tombstones the stub. Shown only when the person's status is `provisional`.
export function MergeProvisional({ person }: { person: Person }) {
  const router = useRouter();
  const { busy, err, run } = useRun();
  const [intoId, setIntoId] = useState("");
  const [confidence, setConfidence] = useState("confirmed");
  return (
    <div className="space-y-3">
      <p className="text-sm text-zinc-500">
        <T>This is a provisional stub. Merge it into a canonical person to re-home its edges and resolve it.</T>
      </p>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_auto_auto]">
        <input
          className="input"
          placeholder="canonical person RID"
          value={intoId}
          onChange={(e) => setIntoId(e.target.value)}
        />
        <select className="input" value={confidence} onChange={(e) => setConfidence(e.target.value)}>
          <option value="confirmed">confirmed</option>
          <option value="probable">probable</option>
          <option value="possible">possible</option>
        </select>
        <button
          type="button"
          className="btn-primary"
          disabled={busy || !intoId.trim()}
          onClick={() =>
            run(
              () => api.person.mergePerson(person.id, { intoPersonId: intoId.trim(), confidence }),
              () => router.push(`/persons/${intoId.trim()}`),
            )
          }
        >
          {busy ? <T>Merging…</T> : <T>Merge</T>}
        </button>
      </div>
      {err ? <ErrorBox error={err} /> : null}
    </div>
  );
}

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
            run(() => api.person.deactivatePerson(person.id, {}))
          }
        >
          <T>Deactivate</T>
        </button>
      ) : (
        <>
          <button
            type="button"
            className="btn-ghost"
            disabled={busy}
            onClick={() => run(() => api.person.reactivatePerson(person.id))}
          >
            <T>Reactivate</T>
          </button>
          <button
            type="button"
            className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
            disabled={busy}
            onClick={() =>
              window.confirm(
                "Purge this person? This irreversibly erases PII (only allowed after the grace window).",
              ) && run(() => api.person.purgePerson(person.id))
            }
          >
            <T>Purge</T>
          </button>
        </>
      )}
      {err ? <ErrorBox error={err} /> : null}
    </div>
  );
}

// useRankIndex fetches the rank scheme once (module-cached) and flattens it to the option list plus a
// rankId -> label lookup, so both SetRank (the editor) and PersonRankLabel (the read-only display)
// resolve the same human labels. Labels are "<system> · <rank>"; re-derived per locale.
let rankSchemeCache: unknown | null = null;
function useRankIndex(): { options: { id: string; label: string }[]; labelOf: (id?: string) => string } {
  const { locale } = useLocale();
  const [scheme, setScheme] = useState<unknown | null>(rankSchemeCache);

  useEffect(() => {
    if (rankSchemeCache) {
      setScheme(rankSchemeCache);
      return;
    }
    api
      .request("GET", "/rank/v1/rank-scheme")
      .then((d) => {
        rankSchemeCache = d;
        setScheme(d);
      })
      .catch(() => {});
  }, []);

  const options: { id: string; label: string }[] = [];
  // The scheme tree is system → category → type (which may nest sub-types) → rank.
  const walkType = (t: { name?: LocaleMap; code: string; children?: unknown[]; ranks?: unknown[] }, sys: string) => {
    for (const rk of (t.ranks as { id: string; name?: LocaleMap; code: string }[]) ?? [])
      options.push({ id: rk.id, label: `${sys} · ${pickLabel(rk.name, locale) || rk.code}` });
    for (const c of (t.children as typeof t[]) ?? []) walkType(c, sys);
  };
  for (const sysNode of (scheme as { systems?: { name?: LocaleMap; code: string; categories?: { types?: unknown[] }[] }[] })?.systems ?? []) {
    const sysLabel = pickLabel(sysNode.name, locale) || sysNode.code;
    for (const c of sysNode.categories ?? [])
      for (const t of (c.types as Parameters<typeof walkType>[0][]) ?? []) walkType(t, sysLabel);
  }
  const index = new Map(options.map((o) => [o.id, o.label]));
  return { options, labelOf: (id) => (id ? index.get(id) ?? "" : "") };
}

/** Read-only display of the rank(s) a person holds (one per system — D-Rank), resolved to labels. */
export function PersonRankLabel({ ranks }: { ranks?: PersonRank[] }) {
  const { labelOf } = useRankIndex();
  if (!ranks || ranks.length === 0) return <span className="text-slate-400">—</span>;
  return (
    <span>
      {ranks
        .map((r) => labelOf(r.rankId) || r.rankId)
        .join(", ")}
    </span>
  );
}

/** Set or clear the person's single rank (flattened from the rank scheme). */
export function SetRank({ personId, ranks }: { personId: string; ranks?: PersonRank[] }) {
  const { busy, err, run } = useRun();
  const tr = useTg();
  const { options } = useRankIndex();
  const [value, setValue] = useState(ranks?.[0]?.rankId ?? "");

  // Keep the editor in sync once the person's held ranks arrive / change.
  useEffect(() => {
    setValue(ranks?.[0]?.rankId ?? "");
  }, [ranks]);

  return (
    <div className="flex items-end gap-2">
      <div className="flex-1">
        <label className="label"><T>Rank</T></label>
        <select className="input" value={value} onChange={(e) => setValue(e.target.value)}>
          <option value="">{tr("— none —")}</option>
          {options.map((r) => (
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
            api.person.setRank(personId, {
              rankId: value || undefined,
              // On clear (no rank chosen) the system must be supplied; reuse the currently held one.
              systemId: value ? undefined : ranks?.[0]?.systemId,
            }),
          )
        }
      >
        {busy ? "…" : <T>Set rank</T>}
      </button>
      {err ? <ErrorBox error={err} /> : null}
    </div>
  );
}

/* ------------------------------------------------------------------ contact channels */

export function EmailManager({ personId, emails }: { personId: string; emails?: Email[] }) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  const types = useCatalog<ContactType>("/person/v1/person/email-types");
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
              api.person.upsertEmail(personId, {
                address: s(f, "address")!,
                typeCode: s(f, "typeCode")!,
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
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

export function PhoneManager({ personId, phones }: { personId: string; phones?: Phone[] }) {
  const { locale } = useLocale();
  const { busy, err, run } = useRun();
  const types = useCatalog<ContactType>("/person/v1/person/phone-types");
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
              api.person.upsertPhone(personId, {
                number: s(f, "number")!,
                typeCode: s(f, "typeCode")!,
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
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

export function CallSignManager({ personId, callSigns }: { personId: string; callSigns?: CallSign[] }) {
  const { busy, err, run } = useRun();
  const tr = useTg();
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
              api.person.upsertCallSign(personId, {
                callSign: s(f, "callSign")!,
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="callSign" required className="input" placeholder={tr("call sign")} />
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
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
  const tr = useTg();
  const countryCode = useCountryMap();
  return (
    <ChannelBlock title="Citizenships" err={err}>
      <ItemList
        items={citizenships}
        render={(c) => `${countryCode(c.country) || c.country}${c.isPrimary ? " (primary)" : ""}${c.basis ? ` · ${c.basis}` : ""}`}
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
              api.person.upsertCitizenship(personId, {
                country: s(f, "country")!,
                basis: s(f, "basis"),
                isPrimary: f.get("isPrimary") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <CountrySelect name="country" required includeEmpty={false} />
        <select name="basis" className="input" defaultValue="">
          <option value="">{tr("basis…")}</option>
          <option value="birth">{tr("birth")}</option>
          <option value="descent">{tr("descent")}</option>
          <option value="naturalization">{tr("naturalization")}</option>
          <option value="other">{tr("other")}</option>
        </select>
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
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
  const tr = useTg();
  const countryCode = useCountryMap();
  return (
    <ChannelBlock title="Residences" err={err}>
      <ItemList
        items={residences}
        render={(r) => [countryCode(r.country) || r.country, r.region].filter(Boolean).join(" / ")}
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
              api.person.upsertResidence(personId, {
                country: s(f, "country")!,
                region: s(f, "region"),
                validFrom: s(f, "validFrom") ?? new Date().toISOString().slice(0, 10),
              }),
            () => form.reset(),
          );
        }}
      >
        <CountrySelect name="country" required includeEmpty={false} />
        <input name="region" className="input" placeholder={tr("region (optional)")} />
        <input name="validFrom" type="date" className="input" />
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ addresses (M32) */

const ADDRESS_ROLES = ["home", "work", "mailing", "other"] as const;

// AddressManager — a person's precise, effective-dated addresses over the M19 Location entity
// (D-PersonAddresses, M32). Self-fetches (addresses are a separate endpoint, not embedded on Person);
// the location is chosen via the shared location typeahead. One primary per person is server-enforced.
export function AddressManager({ personId }: { personId: string }) {
  const tr = useTg();
  const [addrs, setAddrs] = useState<Address[] | null>(null);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const load = () => {
    api.person.listAddresses(personId).then(setAddrs).catch(setErr);
  };
  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [personId]);

  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      after?.();
      load();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <ChannelBlock title="Addresses" err={err}>
      <ItemList
        items={addrs ?? undefined}
        render={(a) => {
          const period = [a.validFrom, a.validTo].filter(Boolean).join(" → ");
          return (
            <span className="inline-flex flex-wrap items-center gap-x-1">
              {a.isPrimary ? <span title={tr("primary")}>★</span> : null}
              <span className="font-medium">{a.role}</span>
              <span className="mx-1 text-slate-300">·</span>
              <span className="font-mono text-xs text-slate-500">{ridTail(a.locationId)}</span>
              {period ? <span className="ml-1 text-xs text-slate-400">({period})</span> : null}
              {a.privacySeeking ? <span className="ml-1" title={tr("privacy-seeking")}>🔒</span> : null}
            </span>
          );
        }}
        del={(a) => `/person/v1/persons/${personId}/addresses/${a.id}`}
        delConfirm="Remove this address?"
      />
      <form
        className="mt-2 grid grid-cols-[1fr_6rem_8rem_auto_auto_auto] items-center gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              api.person.upsertAddress(personId, {
                locationId: s(f, "locationId")!,
                role: s(f, "role") ?? "home",
                validFrom: s(f, "validFrom"),
                isPrimary: f.get("isPrimary") === "on",
                privacySeeking: f.get("privacySeeking") === "on",
              }),
            () => form.reset(),
          );
        }}
      >
        <SearchSelect kind="location" name="locationId" required placeholder="Search a location…" />
        <select name="role" className="input" defaultValue="home">
          {ADDRESS_ROLES.map((r) => (
            <option key={r} value={r}>{r}</option>
          ))}
        </select>
        <input name="validFrom" type="date" className="input" />
        <label className="flex items-center gap-1 text-xs text-slate-500">
          <input name="isPrimary" type="checkbox" /> <T>primary</T>
        </label>
        <label className="flex items-center gap-1 text-xs text-slate-500">
          <input name="privacySeeking" type="checkbox" /> <T>private</T>
        </label>
        <button className="btn-ghost" disabled={busy}><T>Add</T></button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ name variants */

const ALIAS_KINDS = ["aka", "former_legal", "maiden", "pseudonym", "cover"] as const;

export function NameVariantManager({
  personId,
  variants,
}: {
  personId: string;
  variants?: NameVariant[];
}) {
  const { busy, err, run } = useRun();
  const tr = useTg();
  const isTranslit = (n: NameVariant) => !n.variantKind || n.variantKind === "transliteration";
  const translits = (variants ?? []).filter(isTranslit);
  const aliases = (variants ?? []).filter((n) => !isTranslit(n));
  return (
    <ChannelBlock title="Name variants" err={err}>
      <ItemList
        items={translits}
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
              api.person.upsertNameVariant(personId, {
                locale: s(f, "locale")!,
                displayName: s(f, "displayName")!,
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="locale" required className="input" placeholder="ukr" />
        <input name="displayName" required className="input" placeholder="Іван Петренко" />
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
        </button>
      </form>

      <div className="mt-3 text-xs font-medium uppercase tracking-wide text-slate-400">{tr("Aliases")}</div>
      <ItemList
        items={aliases}
        render={(n) => `${n.variantKind} · ${n.locale}: ${n.displayName ?? ""}`}
        del={(n) => `/person/v1/persons/${personId}/name-aliases/${n.id}`}
        delConfirm="Remove this alias?"
      />
      <form
        className="mt-2 grid grid-cols-[5rem_8rem_1fr_auto] gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const form = ev.currentTarget;
          run(
            () =>
              api.person.addNameAlias(personId, {
                locale: s(f, "locale")!,
                variantKind: s(f, "variantKind")!,
                displayName: s(f, "displayName")!,
              }),
            () => form.reset(),
          );
        }}
      >
        <input name="locale" required className="input" placeholder="eng" />
        <select name="variantKind" className="input" defaultValue="aka">
          {ALIAS_KINDS.map((k) => (
            <option key={k} value={k}>{k}</option>
          ))}
        </select>
        <input name="displayName" required className="input" placeholder={tr("alias name")} />
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ physical identity (M31) */

// PhysicalIdentityManager owns the M31 physical-identity panels: effective-dated physical descriptions,
// distinguishing marks (pii:special ceiling) and the GDPR Art. 9 self-declared ethnicity (envelope-
// encrypted server-side, lawful-basis-gated). Each section fetches on mount via the typed SDK.
export function PhysicalIdentityManager({ personId }: { personId: string }) {
  const { locale } = useLocale();
  const tr = useTg();
  const [descs, setDescs] = useState<PhysicalDescription[] | null>(null);
  const [marks, setMarks] = useState<DistinguishingMark[] | null>(null);
  const [eths, setEths] = useState<Ethnicity[] | null>(null);
  const [bases, setBases] = useState<{ code: string; name: string }[]>([]);
  const [colorsById, setColorsById] = useState<Record<string, { name?: Record<string, string>; code: string; hex?: string | null }>>({});
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const load = () => {
    api.person.listPhysicalDescriptions(personId).then(setDescs).catch(setErr);
    api.person.listDistinguishingMarks(personId).then(setMarks).catch(setErr);
    api.person.listEthnicities(personId).then(setEths).catch(setErr);
  };
  // colorChip renders a swatch + localized name for an eye/hair color RID (D-Color); "" -> null.
  const colorChip = (id?: string | null) => {
    if (!id) return null;
    const c = colorsById[id];
    const lbl = c ? pickLabel(c.name, locale) || c.code : id;
    return (
      <span className="inline-flex items-center gap-1">
        <span className="inline-block h-3 w-3 rounded-sm border border-slate-300" style={{ backgroundColor: c?.hex ?? "transparent" }} />
        {lbl}
      </span>
    );
  };
  useEffect(() => {
    load();
    // D-Color: load the eye + hair palettes to resolve description color RIDs to swatch+label.
    Promise.all([api.platformCatalog.listColors("eye"), api.platformCatalog.listColors("hair")])
      .then(([e, h]) => {
        const m: Record<string, { name?: Record<string, string>; code: string; hex?: string | null }> = {};
        for (const c of [...(e?.colors ?? []), ...(h?.colors ?? [])]) m[c.id] = { name: c.name, code: c.code, hex: c.hex };
        setColorsById(m);
      })
      .catch(setErr);
    // Art. 9 special-category conditions only.
    api.platformCatalog.listLegalBasisKinds()
      .then((r) => setBases((r?.kinds ?? []).filter((k) => k.article === "art9").map((k) => ({ code: k.code, name: k.name }))))
      .catch(setErr);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [personId]);

  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      after?.();
      load();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <ChannelBlock title="Physical description" err={err}>
        <ItemList
          items={descs ?? undefined}
          render={(d) => {
            const parts: React.ReactNode[] = [
              d.heightCm ? `${d.heightCm} cm` : null,
              d.weightKg ? `${d.weightKg} kg` : null,
              colorChip(d.eyeColorId),
              colorChip(d.hairColorId),
              d.build || null,
              d.bloodType ? `🩸 ${d.bloodType}` : null,
            ].filter(Boolean);
            if (parts.length === 0) return "—";
            return (
              <span className="inline-flex flex-wrap items-center gap-x-1">
                {parts.map((p, i) => (
                  <span key={i} className="inline-flex items-center">
                    {i > 0 ? <span className="mx-1 text-slate-300">·</span> : null}
                    {p}
                  </span>
                ))}
              </span>
            );
          }}
          del={(d) => `/person/v1/persons/${personId}/physical-descriptions/${d.id}`}
          delConfirm="Remove this description?"
        />
        <form
          className="mt-2 grid grid-cols-[5rem_5rem_1fr_1fr_6rem_auto] gap-2"
          onSubmit={(ev) => {
            ev.preventDefault();
            const f = new FormData(ev.currentTarget);
            const form = ev.currentTarget;
            const num = (k: string) => (s(f, k) ? Number(s(f, k)) : undefined);
            run(
              () =>
                api.person.upsertPhysicalDescription(personId, {
                  heightCm: num("heightCm"),
                  weightKg: num("weightKg"),
                  eyeColorId: s(f, "eyeColorId") || undefined,
                  hairColorId: s(f, "hairColorId") || undefined,
                  bloodType: s(f, "bloodType"),
                }),
              () => form.reset(),
            );
          }}
        >
          <input name="heightCm" type="number" className="input" placeholder={tr("height")} />
          <input name="weightKg" type="number" className="input" placeholder={tr("weight")} />
          <ColorPicker domain="eye" name="eyeColorId" placeholder="eyes" />
          <ColorPicker domain="hair" name="hairColorId" placeholder="hair" />
          <select name="bloodType" className="input" defaultValue="">
            <option value="">{tr("blood…")}</option>
            {["A+", "A-", "B+", "B-", "AB+", "AB-", "O+", "O-", "unknown"].map((b) => (
              <option key={b} value={b}>{b}</option>
            ))}
          </select>
          <button className="btn-ghost" disabled={busy}><T>Add</T></button>
        </form>
      </ChannelBlock>

      <ChannelBlock title="Distinguishing marks" err={null}>
        <ItemList
          items={marks ?? undefined}
          render={(m) => `${m.kind}${m.bodyLocation ? ` · ${m.bodyLocation}` : ""}${m.description ? ` — ${m.description}` : ""}`}
          del={(m) => `/person/v1/persons/${personId}/distinguishing-marks/${m.id}`}
          delConfirm="Remove this mark?"
        />
        <form
          className="mt-2 grid grid-cols-[7rem_1fr_1fr_auto] gap-2"
          onSubmit={(ev) => {
            ev.preventDefault();
            const f = new FormData(ev.currentTarget);
            const form = ev.currentTarget;
            run(
              () =>
                api.person.upsertDistinguishingMark(personId, {
                  kind: s(f, "kind")!,
                  bodyLocation: s(f, "bodyLocation"),
                  description: s(f, "description"),
                }),
              () => form.reset(),
            );
          }}
        >
          <select name="kind" className="input" defaultValue="tattoo">
            {["tattoo", "scar", "piercing", "birthmark"].map((k) => (
              <option key={k} value={k}>{k}</option>
            ))}
          </select>
          <input name="bodyLocation" className="input" placeholder={tr("location")} />
          <input name="description" className="input" placeholder={tr("description")} />
          <button className="btn-ghost" disabled={busy}><T>Add</T></button>
        </form>
      </ChannelBlock>

      <ChannelBlock title="Ethnicity" err={null}>
        <p className="mt-1 text-xs text-amber-600"><T>Special-category data (GDPR Art. 9) — self-declared, encrypted at rest.</T></p>
        <ul className="mt-1 space-y-0.5 text-sm text-slate-700">
          {(eths ?? []).map((e) => (
            <li key={e.id} className="flex items-center justify-between gap-2">
              <span>
                {e.name || e.code || "—"}
                <span className="ml-1 text-xs text-slate-400">· {e.legalBasis}</span>
              </span>
              <RowDelete path={`/person/v1/persons/${personId}/ethnicities/${e.id}`} confirm="Remove this ethnicity?" />
            </li>
          ))}
        </ul>
        <form
          className="mt-2 grid grid-cols-[1fr_1fr_auto] gap-2"
          onSubmit={(ev) => {
            ev.preventDefault();
            const f = new FormData(ev.currentTarget);
            const code = s(f, "code");
            const legalBasis = s(f, "legalBasis");
            if (!code || !legalBasis) return;
            const form = ev.currentTarget;
            run(() => api.person.addEthnicity(personId, { code, legalBasis }), () => form.reset());
          }}
        >
          <EthnicityPicker name="code" placeholder="ethnicity…" />
          <select name="legalBasis" className="input" defaultValue="explicit_consent" required>
            {bases.map((b) => (
              <option key={b.code} value={b.code}>{b.name}</option>
            ))}
          </select>
          <button className="btn-ghost" disabled={busy}><T>Add</T></button>
        </form>
      </ChannelBlock>
    </>
  );
}

/* ------------------------------------------------------------------ documents / personal codes */

export function DocumentManager({
  personId,
  documents,
}: {
  personId: string;
  documents?: DocumentDoc[];
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const { busy, err, run } = useRun();
  const countryCode = useCountryMap();
  const types = useCatalog<Catalog>("/document/v1/document-types");
  return (
    <ChannelBlock title="Documents" err={err}>
      <ItemList
        items={documents}
        render={(d) => `${d.number ?? d.id.slice(-8)} · ${countryCode(d.issuingCountry ?? undefined) || d.issuingCountry || ""} · ${d.status ?? ""}`}
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
              api.document.attachDocument(personId, {
                typeId: s(f, "typeId")!,
                number: s(f, "number"),
                issuingCountry: s(f, "issuingCountry"),
              }),
            () => form.reset(),
          );
        }}
      >
        <select name="typeId" required className="input" defaultValue="">
          <option value="" disabled>
            {tr("type…")}
          </option>
          {types.map((t) => (
            <option key={t.id} value={t.id}>
              {pickLabel(t.name, locale) || t.code}
            </option>
          ))}
        </select>
        <input name="number" className="input" placeholder={tr("number")} />
        <CountrySelect name="issuingCountry" />
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

export function PersonalCodeManager({
  personId,
  codes,
}: {
  personId: string;
  codes?: CodeRow[];
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const { busy, err, run } = useRun();
  const schemes = useCatalog<{ code: string; name?: LocaleMap }>("/document/v1/personal-code-schemes");
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
              api.document.attachPersonalCode(personId, {
                schemeCode: s(f, "schemeCode")!,
                value: s(f, "value")!,
              }),
            () => form.reset(),
          );
        }}
      >
        <select name="schemeCode" required className="input" defaultValue="">
          <option value="" disabled>
            {tr("scheme…")}
          </option>
          {schemes.map((sch) => (
            <option key={sch.code} value={sch.code}>
              {pickLabel(sch.name, locale) || sch.code}
            </option>
          ))}
        </select>
        <input name="value" required className="input" placeholder={tr("identifier value")} />
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
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
}: {
  personId: string;
  accounts?: SocialAccount[];
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const { busy, err, run } = useRun();
  const platforms = useCatalog<Platform>("/person/v1/person/platforms");
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
              api.person.upsertSocialAccount(personId, {
                platformCode: s(f, "platformCode")!,
                handle: s(f, "handle")!,
                source: s(f, "source")!,
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
            {tr("platform…")}
          </option>
          {platforms.map((p) => (
            <option key={p.code} value={p.code}>
              {pickLabel(p.name, locale) || p.code}
            </option>
          ))}
        </select>
        <input name="handle" required className="input" placeholder={tr("handle")} />
        <select name="source" required className="input" defaultValue="self_declared">
          <option value="self_declared">{tr("self-declared")}</option>
          <option value="operator_verified">{tr("operator-verified")}</option>
          <option value="imported">{tr("imported")}</option>
        </select>
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
        </button>
        <label className="col-span-4 flex items-center gap-3 text-xs text-slate-500">
          <span className="inline-flex items-center gap-1">
            <input type="checkbox" name="platformVerified" /> {tr("platform-verified")}
          </span>
          <span className="inline-flex items-center gap-1">
            <input type="checkbox" name="isPrimary" /> {tr("primary")}
          </span>
        </label>
      </form>
    </ChannelBlock>
  );
}

/** Inline disclosure that lazy-loads a social account's @handle rename history. */
function HandleHistory({ personId, accountId }: { personId: string; accountId: string }) {
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [rows, setRows] = useState<SocialAccountHandle[] | null>(null);
  const toggle = () => {
    const next = !open;
    setOpen(next);
    if (next && rows === null)
      api.person.listSocialAccountHandles(personId, accountId)
        .then(setRows)
        .catch(() => setRows([]));
  };
  return (
    <>
      <button type="button" className="text-indigo-600 hover:underline" onClick={toggle}>
        · {open ? tr("hide") : tr("history")}
      </button>
      {open ? (
        <ul className="mt-1 w-full pl-3 text-xs text-slate-500">
          {rows === null ? (
            <li>{tr("loading…")}</li>
          ) : rows.length === 0 ? (
            <li>{tr("no rename history")}</li>
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
  emails,
  phones,
}: {
  personId: string;
  links?: MessengerLink[];
  emails?: Email[];
  phones?: Phone[];
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const { busy, err, run } = useRun();
  const platforms = useCatalog<Platform>("/person/v1/person/platforms");
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
              api.person.upsertMessengerLink(personId, {
                platformCode: s(f, "platformCode")!,
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
            {tr("messenger…")}
          </option>
          {messengers.map((p) => (
            <option key={p.code} value={p.code}>
              {pickLabel(p.name, locale) || p.code}
            </option>
          ))}
        </select>
        <select name="channel" required className="input" defaultValue="">
          <option value="" disabled>
            {tr("phone or email…")}
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
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ relationships (M14) */

// One row per relation family, sharing the ChannelBlock + EntitySelect + status pattern. Each family
// upserts to its own collection; all share the DELETE .../relationships/{id} sink. The counterpart
// person is picked via EntitySelect; relation-code <select>s are fed by category-filtered RelationTypes.
// counterpartId is the RID of the OTHER person (resolved to a clickable name by PersonLink); prefix is
// an optional role qualifier ("child" / "guardian" / …) shown ahead of the name.
type RelRow = { id: string; counterpartId: string; prefix?: string; sub: string; tone?: "green" | "amber" | "slate" };

export function RelationshipManager({
  personId,
  partnerships,
  kinships,
  guardianships,
  sponsorships,
  nextOfKin,
  associations,
}: {
  personId: string;
  partnerships?: Partnership[];
  kinships?: Kinship[];
  guardianships?: Guardianship[];
  sponsorships?: Sponsorship[];
  nextOfKin?: NextOfKin[];
  associations?: Association[];
}) {
  const relationTypes = useCatalog<RelationType>("/person/v1/person/relation-types");
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
          counterpartId: other(r.personIdA, r.personIdB),
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
          counterpartId: other(r.parentId, r.childId),
          prefix: r.parentId === personId ? "child" : "parent",
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
          counterpartId: other(r.guardianId, r.wardId),
          prefix: r.guardianId === personId ? "ward" : "guardian",
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
          counterpartId: other(r.sponsorId, r.sponsoredId),
          prefix: r.sponsorId === personId ? "sponsored" : "sponsor",
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
          counterpartId: other(r.subjectId, r.contactId),
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
          counterpartId: other(r.personIdA, r.personIdB),
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
  const tr = useTg();
  const opts = types.filter((t) => t.category === category);
  return (
    <select name="relationCode" required={required} className="input" defaultValue="">
      <option value="">{required ? tr("relation…") : tr("relation (optional)")}</option>
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
              <span className="flex items-center gap-2 text-xs">
                {r.prefix ? <span className="text-slate-400">{r.prefix}:</span> : null}
                <PersonLink personId={r.counterpartId} />
                <span className="rounded-full bg-slate-100 px-1.5 text-slate-500">{r.sub}</span>
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
          run(() => api.request("PUT", `/person/v1/persons/${personId}${upsertPath}`, { body }), () => form.reset());
        }}
      >
        <div className="min-w-[14rem] flex-1">
          <EntitySelect kind="person" name={counterpartField} required placeholder="counterpart person…" />
        </div>
        {extra}
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ small shared UI */

/* ------------------------------------------------------------------ languages (D-Languages, M18) */

// PersonLanguageManager owns the SPEAKS sub-resource. Unlike the embedded channels, person_languages
// is not part of the Person aggregate, so it fetches its own list and refreshes it after each write.
export function PersonLanguageManager({ personId }: { personId: string }) {
  const { locale } = useLocale();
  const tr = useTg();
  const [rows, setRows] = useState<PersonLanguage[] | null>(null);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [langId, setLangId] = useState("");
  const [pickerKey, setPickerKey] = useState(0);

  const load = () =>
    api.person.listPersonLanguages(personId)
      .then((r) => setRows(r ?? []))
      .catch(setErr);
  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [personId]);

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
    <ChannelBlock title="Languages spoken" err={err}>
      {rows && rows.length === 0 ? <p className="mt-1 text-sm text-slate-400">—</p> : null}
      <ul className="mt-1 space-y-0.5 text-sm text-slate-700">
        {(rows ?? []).map((l) => (
          <li key={l.id} className="flex items-center justify-between gap-2">
            <span>
              {pickLabel(l.name, locale) || l.languageId}
              {l.isNative ? tr(" · native") : ""}
              {l.cefrLevel ? ` · ${l.cefrLevel}` : ""}
            </span>
            <button
              type="button"
              className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
              disabled={busy}
              onClick={() =>
                window.confirm("Remove this language?") &&
                run(() => api.person.deletePersonLanguage(personId, l.languageId))
              }
            >
              <T>Remove</T>
            </button>
          </li>
        ))}
      </ul>
      <form
        className="mt-2 grid grid-cols-[1fr_6rem_auto_auto] items-center gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          if (!langId) return;
          const f = new FormData(ev.currentTarget);
          run(
            () =>
              api.person.upsertPersonLanguage(personId, {
                languageId: langId,
                cefrLevel: s(f, "cefrLevel"),
                isNative: f.get("isNative") === "on",
              }),
            () => {
              setLangId("");
              setPickerKey((k) => k + 1);
            },
          );
        }}
      >
        <LanguagePicker key={pickerKey} onChange={setLangId} />
        <select name="cefrLevel" className="input" defaultValue="">
          <option value="">{tr("CEFR…")}</option>
          {["A1", "A2", "B1", "B2", "C1", "C2"].map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        <label className="flex items-center gap-1 text-xs text-slate-600">
          <input type="checkbox" name="isNative" /> {tr("native")}
        </label>
        <button className="btn-ghost" disabled={busy || !langId}>
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ clergy credentials (D-ClergyCredential, M23) */

// PersonClergyManager owns the public person↔religion ordination links. Add cites a grade + the
// conferring org unit; revocation is a status flip (never a delete — indelible where sacramental).
export function PersonClergyManager({ personId }: { personId: string }) {
  const { locale } = useLocale();
  const tr = useTg();
  const [rows, setRows] = useState<ClergyCredential[] | null>(null);
  const [grades, setGrades] = useState<ClergyGrade[]>([]);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [orgUnitId, setOrgUnitId] = useState("");
  const [unitKey, setUnitKey] = useState(0);

  const load = () =>
    api.religion.listPersonClergyCredentials(personId)
      .then((r) => setRows(r?.credentials ?? []))
      .catch(setErr);
  useEffect(() => {
    load();
    api.religion.listClergyGrades()
      .then((r) => setGrades(r?.clergyGrades ?? []))
      .catch(setErr);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [personId]);

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
    <ChannelBlock title="Clergy credentials" err={err}>
      {rows && rows.length === 0 ? <p className="mt-1 text-sm text-slate-400">—</p> : null}
      <ul className="mt-1 space-y-0.5 text-sm text-slate-700">
        {(rows ?? []).map((c) => (
          <li key={c.id} className="flex items-center justify-between gap-2">
            <span>
              {pickLabel(c.gradeName, locale) || c.gradeCode}
              {" · "}
              <span className="font-mono text-xs">{c.orgUnitId.slice(-8)}</span>
              {c.grantedOn ? ` · ${c.grantedOn}` : ""}
              <span
                className={
                  "ml-1 rounded-full px-1.5 text-xs " +
                  (c.status === "active" ? "bg-green-100 text-green-700" : "bg-slate-100 text-slate-500")
                }
              >
                {c.status}
              </span>
            </span>
            <span className="flex gap-2">
              {c.status !== "suspended" ? (
                <button
                  type="button"
                  className="text-xs font-medium text-amber-600 hover:underline disabled:opacity-50"
                  disabled={busy}
                  onClick={() =>
                    run(() => api.religion.updateClergyCredential(c.id, { status: "suspended" as never }))
                  }
                >
                  <T>Suspend</T>
                </button>
              ) : null}
              {c.status !== "revoked" ? (
                <button
                  type="button"
                  className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
                  disabled={busy}
                  onClick={() =>
                    window.confirm("Revoke this credential? (indelible — recorded as a status flip)") &&
                    run(() => api.religion.updateClergyCredential(c.id, { status: "revoked" as never }))
                  }
                >
                  <T>Revoke</T>
                </button>
              ) : null}
              {c.status !== "active" ? (
                <button
                  type="button"
                  className="text-xs font-medium text-green-600 hover:underline disabled:opacity-50"
                  disabled={busy}
                  onClick={() =>
                    run(() => api.religion.updateClergyCredential(c.id, { status: "active" as never }))
                  }
                >
                  <T>Reinstate</T>
                </button>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
      <form
        className="mt-2 grid grid-cols-[1fr_1fr_8rem_auto] items-center gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const gradeId = s(f, "clergyGradeId");
          if (!gradeId || !orgUnitId) return;
          run(
            () =>
              api.religion.addClergyCredential(personId, {
                clergyGradeId: gradeId,
                orgUnitId,
                grantedOn: s(f, "grantedOn"),
              }),
            () => {
              setOrgUnitId("");
              setUnitKey((k) => k + 1);
              (ev.target as HTMLFormElement).reset();
            },
          );
        }}
      >
        <select name="clergyGradeId" className="input" defaultValue="" required>
          <option value="">{tr("grade…")}</option>
          {grades.map((g) => (
            <option key={g.id} value={g.id}>
              {pickLabel(g.name, locale) || g.code}
            </option>
          ))}
        </select>
        <EntitySelect key={unitKey} kind="unit" onChange={setOrgUnitId} placeholder="conferring org unit…" />
        <input name="grantedOn" type="date" className="input" />
        <button className="btn-ghost" disabled={busy || !orgUnitId}>
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

/* ------------------------------------------------------------------ lay affiliation (D-ReligiousAffiliation, M24, pii:special) */

// PersonAffiliationManager owns the GDPR Art. 9 person↔religion lay-affiliation links. The belief
// `value` is envelope-encrypted server-side; the API returns it decrypted to authorized readers.
export function PersonAffiliationManager({ personId }: { personId: string }) {
  const { locale } = useLocale();
  const tr = useTg();
  const [rows, setRows] = useState<Affiliation[] | null>(null);
  const [types, setTypes] = useState<AffiliationType[]>([]);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [religionId, setReligionId] = useState("");
  const [pickerKey, setPickerKey] = useState(0);

  const load = () =>
    api.religion.listPersonAffiliations(personId)
      .then((r) => setRows(r?.affiliations ?? []))
      .catch(setErr);
  useEffect(() => {
    load();
    api.religion.listAffiliationTypes()
      .then((r) => setTypes(r?.affiliationTypes ?? []))
      .catch(setErr);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [personId]);

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
    <ChannelBlock title="Religious affiliation" err={err}>
      <p className="mt-1 text-xs text-amber-600"><T>Special-category data (GDPR Art. 9) — encrypted at rest.</T></p>
      {rows && rows.length === 0 ? <p className="mt-1 text-sm text-slate-400">—</p> : null}
      <ul className="mt-1 space-y-0.5 text-sm text-slate-700">
        {(rows ?? []).map((a) => (
          <li key={a.id} className="flex items-center justify-between gap-2">
            <span>
              {pickLabel(a.affiliationTypeName, locale) || a.affiliationTypeCode}
              {a.value ? ` · ${a.value}` : ""}
              <span
                className={
                  "ml-1 rounded-full px-1.5 text-xs " +
                  (a.status === "active" ? "bg-green-100 text-green-700" : "bg-slate-100 text-slate-500")
                }
              >
                {a.status}
              </span>
            </span>
            <button
              type="button"
              className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50"
              disabled={busy}
              onClick={() =>
                window.confirm("Remove this affiliation?") &&
                run(() => api.religion.deleteAffiliation(a.id))
              }
            >
              <T>Remove</T>
            </button>
          </li>
        ))}
      </ul>
      <form
        className="mt-2 grid grid-cols-[1fr_1fr_1fr_auto] items-center gap-2"
        onSubmit={(ev) => {
          ev.preventDefault();
          const f = new FormData(ev.currentTarget);
          const typeId = s(f, "affiliationTypeId");
          if (!typeId) return;
          run(
            () =>
              api.religion.addAffiliation(personId, {
                affiliationTypeId: typeId,
                religionId: religionId || undefined,
                value: s(f, "value"),
              }),
            () => {
              setReligionId("");
              setPickerKey((k) => k + 1);
              (ev.target as HTMLFormElement).reset();
            },
          );
        }}
      >
        <select name="affiliationTypeId" className="input" defaultValue="" required>
          <option value="">{tr("type…")}</option>
          {types.map((t) => (
            <option key={t.id} value={t.id}>
              {pickLabel(t.name, locale) || t.code}
            </option>
          ))}
        </select>
        <ReligionTaxonPicker key={pickerKey} value={religionId} onChange={setReligionId} />
        <input name="value" type="text" className="input" placeholder={tr("detail (optional)")} />
        <button className="btn-ghost" disabled={busy}>
          <T>Add</T>
        </button>
      </form>
    </ChannelBlock>
  );
}

function ChannelBlock({
  title,
  err,
  children,
}: {
  title: string;
  err: unknown;
  children: React.ReactNode;
}) {
  const tr = useTg();
  return (
    <div className="mt-3">
      <div className="text-xs font-medium uppercase tracking-wide text-slate-400">{tr(title)}</div>
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
  render: (it: T) => React.ReactNode;
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
