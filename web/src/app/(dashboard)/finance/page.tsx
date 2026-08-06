"use client";

// Finance workspace (M44 / D-Finance). Create bank accounts and drill into one to see and record
// its holders (the polymorphic person|company ownership edges) and its cards. The IBAN/PAN are
// envelope-encrypted at rest; the IBAN is shown (decrypted) only when an account is opened, the full PAN
// only when a card is opened — both for authorized callers. Account-type / card-network catalogs are
// managed here too. A bank is a `company`-domain organization (M21/M41). A person's held accounts are
// surfaced on the person object view.
//
// M58 ticket 3 moved BROWSING out. /explore/account and /explore/card are the registries' real
// readers: facet filters, keyset paging that does not drop its token, and a dashboard over the same
// filter set. `card` got a top-level list in that ticket precisely so it could have one — before it,
// cards were reachable only through their account, so there was no collection to browse or count.
//
// What stays here is EDITING — creation, the account-type/card-network catalogs, the per-account
// holder and card panels, and the IBAN/PAN REVEAL, which is this page's alone: a decrypted
// identifier is fetched one row at a time for an authorized caller and is never listed anywhere,
// explorer included (PCI-DSS Req 3; D-DataScope CDE scope).
//
// The table below is therefore a bounded EDIT surface, not a listing: it shows one page and says so.

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { SearchSelect } from "@/components/SearchSelect";
import { PageHeader, Card, Table, Mono } from "@/components/ui";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { useTg } from "@/lib/locale";
import { newSuffix, slugify } from "@/lib/code";
import { pickLabel, type LocaleMap } from "@/lib/i18n";

type Catalog = { id: string; code: string; name: LocaleMap };
type Account = {
  id: string; institutionId: string; institutionLabel?: string; iban?: string;
  currency?: string; accountTypeId?: string; accountTypeLabel?: string; status: string;
};
type Holder = {
  id: string; accountId: string; holderKind: string; holderId: string; holderLabel?: string;
  role: string; effectiveFrom: string; effectiveTo?: string;
};
type Card = {
  id: string; accountId: string; pan?: string; bin?: string; lastFour?: string;
  networkId?: string; networkLabel?: string; cardType: string; status: string;
};

const label = (m: LocaleMap, fallback: string) => pickLabel(m) || fallback;

// The edit surface's page size. Small on purpose: this is a working set to act on, not a listing —
// the listings are /explore/account and /explore/card.
const EDIT_PAGE = 50;

export default function FinancePage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [accountTypes, setAccountTypes] = useState<Catalog[]>([]);
  const [networks, setNetworks] = useState<Catalog[]>([]);
  const [selected, setSelected] = useState<Account | null>(null);
  const [truncated, setTruncated] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const tr = useTg();

  // One page, and the next-page token is READ rather than discarded — its presence is what the notice
  // below reports. Filtering is deliberately not offered here; it is the explorer's job now.
  function reload() {
    api.finance
      // Positional, and the positions MOVED: M59 inserted the gated `holderKind` filter before
      // `pageSize`, so the old five-argument call silently passed EDIT_PAGE as holderKind. The
      // undefineds are spelled out per parameter rather than counted, so the next inserted arg is a
      // type error here instead of a page size landing in a filter.
      .listAccounts(
        /* institutionId */ undefined,
        /* currency */ undefined,
        /* accountTypeId */ undefined,
        /* status */ undefined,
        /* holderKind */ undefined,
        /* pageSize */ EDIT_PAGE,
      )
      .then((r) => {
        setAccounts((r.accounts ?? []) as unknown as Account[]);
        setTruncated(Boolean(r.nextPageToken));
      })
      .catch(setErr);
  }
  useEffect(() => {
    api.finance.listAccountTypes().then((r) => setAccountTypes((r.types ?? []) as unknown as Catalog[])).catch(() => {});
    api.finance.listCardNetworks().then((r) => setNetworks((r.networks ?? []) as unknown as Catalog[])).catch(() => {});
    reload();
  }, []);

  return (
    <div>
      <PageHeader
        title={<T>Finance</T>}
        description={<T>Bank accounts (envelope-encrypted IBAN) and payment cards (envelope-encrypted PAN, no CVV) as authoritative first-party directory data. A bank is a company organization; ownership is a polymorphic person|company holder link. The IBAN/PAN are shown only when an account/card is opened.</T>}
      />
      {err ? <div className="mb-4"><ErrorBox error={err} /></div> : null}

      <div className="grid gap-6 lg:grid-cols-2">
        <CreateAccount accountTypes={accountTypes} onCreated={reload} setErr={setErr} />
        <CatalogsCard onChanged={() => {
          api.finance.listAccountTypes().then((r) => setAccountTypes((r.types ?? []) as unknown as Catalog[]));
          api.finance.listCardNetworks().then((r) => setNetworks((r.networks ?? []) as unknown as Catalog[]));
        }} setErr={setErr} />
      </div>

      <Card className="mt-6">
        <div className="mb-2 flex items-center gap-3">
          <h2 className="text-sm font-semibold text-slate-900"><T>Edit</T></h2>
          <span className="ml-auto flex items-center gap-3 text-xs">
            <Link href="/explore/account" className="text-indigo-600 hover:underline">
              <T>Browse accounts →</T>
            </Link>
            <Link href="/explore/card" className="text-indigo-600 hover:underline">
              <T>Browse cards →</T>
            </Link>
          </span>
        </div>
        <p className="mb-3 text-xs text-slate-500">
          {truncated
            ? tr("The first page only — there are more. Use the explorer to find a specific account; this table is here to edit the ones in front of you.")
            : tr("Every account in the registry. Use the explorer to filter or chart them.")}
        </p>
        <Table head={<><th className="th"><T>Account</T></th><th className="th"><T>Bank</T></th><th className="th"><T>Type</T></th><th className="th"><T>Currency</T></th><th className="th"><T>Status</T></th><th className="th"></th></>}>
          {accounts.map((a) => (
            <tr key={a.id} className="border-t">
              <td className="py-1"><Mono>{a.id.slice(-8)}</Mono></td>
              <td>{a.institutionLabel || a.institutionId.slice(-8)}</td>
              <td>{a.accountTypeLabel || "—"}</td>
              <td>{a.currency || "—"}</td>
              <td>{a.status}</td>
              <td>
                <span className="flex items-center gap-3">
                  <button className="text-xs text-indigo-600 hover:underline" onClick={() => setSelected(a)}><T>Open</T></button>
                  <button
                    className="text-xs text-red-600 hover:underline"
                    onClick={() => {
                      if (!window.confirm("Delete this account and its cards?")) return;
                      api.finance.deleteAccount(a.id)
                        .then(() => { if (selected?.id === a.id) setSelected(null); reload(); })
                        .catch(setErr);
                    }}
                  >
                    <T>Delete</T>
                  </button>
                </span>
              </td>
            </tr>
          ))}
        </Table>
      </Card>

      {selected ? <AccountDetail account={selected} networks={networks} setErr={setErr} /> : null}
    </div>
  );
}

function CreateAccount({ accountTypes, onCreated, setErr }: { accountTypes: Catalog[]; onCreated: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [institutionId, setInstitutionId] = useState("");
  const [iban, setIban] = useState("");
  const [currency, setCurrency] = useState("");
  const [accountTypeId, setAccountTypeId] = useState("");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!institutionId || !iban.trim()) return;
    api.finance
      .createAccount({ institutionId, iban, currency: currency || undefined, accountTypeId: accountTypeId || undefined })
      .then(() => { setIban(""); setCurrency(""); onCreated(); })
      .catch(setErr);
  }

  return (
    <Card>
      <h2 className="mb-2 text-sm font-semibold text-slate-900"><T>New account</T></h2>
      <form onSubmit={submit} className="space-y-2 text-sm">
        <SearchSelect kind="company" placeholder={tr("Bank (company)…")} onChange={setInstitutionId} />
        <input className="input" placeholder={tr("IBAN")} value={iban} onChange={(e) => setIban(e.target.value)} required />
        <input className="input" placeholder={tr("Currency (ISO 4217, optional)")} value={currency} onChange={(e) => setCurrency(e.target.value.toUpperCase())} maxLength={3} />
        <select className="input" value={accountTypeId} onChange={(e) => setAccountTypeId(e.target.value)}>
          <option value="">— {tr("account type (optional)")} —</option>
          {accountTypes.map((t) => <option key={t.id} value={t.id}>{label(t.name, t.code)}</option>)}
        </select>
        <button className="btn-primary" type="submit"><T>Create</T></button>
      </form>
    </Card>
  );
}

function CatalogsCard({ onChanged, setErr }: { onChanged: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [typeName, setTypeName] = useState("");
  const [networkName, setNetworkName] = useState("");

  function addType(e: React.FormEvent) {
    e.preventDefault();
    if (!typeName.trim()) return;
    api.finance.upsertAccountType({ code: slugify(typeName) + newSuffix(), name: typeName }).then(() => { setTypeName(""); onChanged(); }).catch(setErr);
  }
  function addNetwork(e: React.FormEvent) {
    e.preventDefault();
    if (!networkName.trim()) return;
    api.finance.upsertCardNetwork({ code: slugify(networkName) + newSuffix(), name: networkName }).then(() => { setNetworkName(""); onChanged(); }).catch(setErr);
  }

  return (
    <Card>
      <h2 className="mb-2 text-sm font-semibold text-slate-900"><T>Catalogs</T></h2>
      <form onSubmit={addType} className="mb-3 space-y-2 text-sm">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-400"><T>New account type</T></p>
        <input className="input" placeholder={tr("Account type name")} value={typeName} onChange={(e) => setTypeName(e.target.value)} />
        <button className="btn-secondary" type="submit"><T>Add account type</T></button>
      </form>
      <form onSubmit={addNetwork} className="space-y-2 text-sm">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-400"><T>New card network</T></p>
        <input className="input" placeholder={tr("Card network name")} value={networkName} onChange={(e) => setNetworkName(e.target.value)} />
        <button className="btn-secondary" type="submit"><T>Add card network</T></button>
      </form>
    </Card>
  );
}

function AccountDetail({ account, networks, setErr }: { account: Account; networks: Catalog[]; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [full, setFull] = useState<Account | null>(null);
  const [holders, setHolders] = useState<Holder[]>([]);
  const [cards, setCards] = useState<Card[]>([]);

  function reloadHolders() {
    api.finance.listAccountHolders(account.id).then((r) => setHolders((r.holders ?? []) as unknown as Holder[])).catch(setErr);
  }
  function reloadCards() {
    api.finance.listAccountCards(account.id).then((r) => setCards((r.cards ?? []) as unknown as Card[])).catch(setErr);
  }
  useEffect(() => {
    api.finance.getAccount(account.id).then((a) => setFull(a as unknown as Account)).catch(setErr);
    reloadHolders();
    reloadCards();
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [account.id]);

  return (
    <Card className="mt-6">
      <h2 className="mb-2 text-sm font-semibold text-slate-900">
        <T>Account</T> <Mono>{account.id.slice(-8)}</Mono>
        {full?.iban ? <span className="ml-2 font-mono text-xs text-slate-500">{full.iban}</span> : null}
      </h2>

      <AddHolderForm accountId={account.id} onDone={reloadHolders} setErr={setErr} />

      <h3 className="mb-1 mt-4 text-xs font-semibold uppercase tracking-wide text-slate-400"><T>Holders</T></h3>
      <ul className="space-y-1 text-sm">
        {holders.map((h) => (
          <li key={h.id} className="flex items-center gap-2">
            <span className="text-slate-500">{h.holderKind === "company" ? (h.holderLabel || h.holderId.slice(-8)) : `person ${h.holderId.slice(-8)}`}</span>
            <span className="text-slate-400">· {h.role}</span>
            {!h.effectiveTo ? <button className="text-xs text-indigo-600 hover:underline" onClick={() => api.finance.endAccountHolding(h.id).then(reloadHolders).catch(setErr)}><T>End</T></button> : <span className="text-slate-400">(ended)</span>}
          </li>
        ))}
        {holders.length === 0 ? <li className="text-slate-400"><T>No holders.</T></li> : null}
      </ul>

      <h3 className="mb-1 mt-4 text-xs font-semibold uppercase tracking-wide text-slate-400"><T>Cards</T></h3>
      <AddCardForm accountId={account.id} networks={networks} onDone={reloadCards} setErr={setErr} />
      <ul className="mt-1 space-y-0.5 text-sm">
        {cards.map((c) => (
          <li key={c.id}>
            <span className="font-mono">{c.bin || "••••••"} •••• {c.lastFour || "••••"}</span>
            {c.networkLabel ? ` · ${c.networkLabel}` : ""} · {c.cardType} · {c.status}
            <button className="ml-2 text-xs text-indigo-600 hover:underline" onClick={() => api.finance.getCard(c.id).then((full) => { const p = (full as unknown as Card).pan; if (p) alert(p); }).catch(setErr)}><T>Reveal PAN</T></button>
            <button className="ml-2 text-xs text-red-600 hover:underline" onClick={() => { if (window.confirm("Delete this card?")) api.finance.deleteCard(c.id).then(reloadCards).catch(setErr); }}><T>Delete</T></button>
          </li>
        ))}
        {cards.length === 0 ? <li className="text-slate-400"><T>No cards.</T></li> : null}
      </ul>
      <p className="mt-2 text-xs text-slate-400">{tr("PCI-DSS: storing the full PAN brings the deployment into cardholder-data scope; CVV is never stored.")}</p>
    </Card>
  );
}

function AddHolderForm({ accountId, onDone, setErr }: { accountId: string; onDone: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [holderKind, setHolderKind] = useState<"person" | "company">("person");
  const [holderId, setHolderId] = useState("");
  const [role, setRole] = useState("primary");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!holderId) return;
    api.finance.addAccountHolder(accountId, { holderKind, holderId, role }).then(() => { setHolderId(""); onDone(); }).catch(setErr);
  }
  return (
    <form onSubmit={submit} className="grid gap-2 text-sm sm:grid-cols-3">
      <select className="input" value={holderKind} onChange={(e) => { setHolderKind(e.target.value as "person" | "company"); setHolderId(""); }}>
        <option value="person">person</option>
        <option value="company">company</option>
      </select>
      <SearchSelect kind={holderKind} key={holderKind} placeholder={tr("Holder…")} onChange={setHolderId} />
      <div className="flex gap-2">
        <select className="input" value={role} onChange={(e) => setRole(e.target.value)}>
          <option value="primary">primary</option>
          <option value="joint">joint</option>
          <option value="authorized_signer">authorized signer</option>
        </select>
        <button className="btn-secondary" type="submit"><T>Add</T></button>
      </div>
    </form>
  );
}

function AddCardForm({ accountId, networks, onDone, setErr }: { accountId: string; networks: Catalog[]; onDone: () => void; setErr: (e: unknown) => void }) {
  const tr = useTg();
  const [pan, setPan] = useState("");
  const [networkId, setNetworkId] = useState("");
  const [cardType, setCardType] = useState("debit");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!pan.trim()) return;
    api.finance.addCard(accountId, { pan, networkId: networkId || undefined, cardType }).then(() => { setPan(""); onDone(); }).catch(setErr);
  }
  return (
    <form onSubmit={submit} className="grid gap-2 text-sm sm:grid-cols-3">
      <input className="input" placeholder={tr("Card number (PAN)")} value={pan} onChange={(e) => setPan(e.target.value)} />
      <select className="input" value={networkId} onChange={(e) => setNetworkId(e.target.value)}>
        <option value="">— {tr("network (optional)")} —</option>
        {networks.map((n) => <option key={n.id} value={n.id}>{label(n.name, n.code)}</option>)}
      </select>
      <div className="flex gap-2">
        <select className="input" value={cardType} onChange={(e) => setCardType(e.target.value)}>
          <option value="debit">debit</option>
          <option value="credit">credit</option>
        </select>
        <button className="btn-secondary" type="submit"><T>Add card</T></button>
      </div>
    </form>
  );
}
