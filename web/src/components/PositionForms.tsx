"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import { errorMessage } from "@/lib/api/errors";
import { ErrorBox } from "./ErrorBox";
import { EntitySelect } from "./EntitySelect";
import { ActionButton } from "./ActionButton";
import { T } from "./T";
import { pickLabel } from "@/lib/i18n";
import { useLocale, useTg } from "@/lib/locale";
import { newSuffix, slugify } from "@/lib/code";
import { ridTail } from "@/lib/ontology/rid";
import type { Position } from "@/lib/api/types";

// PersonLink resolves a person RID to a clickable display-name link, caching names module-wide so a
// table of positions doesn't refetch the same holder. Falls back to the RID tail while loading.
const personNameCache = new Map<string, string>();
export function PersonLink({ personId }: { personId: string }) {
  const [name, setName] = useState<string>(() => personNameCache.get(personId) ?? "");
  useEffect(() => {
    if (personNameCache.has(personId)) {
      setName(personNameCache.get(personId) ?? "");
      return;
    }
    let alive = true;
    api.person
      .getPerson(personId)
      .then((p) => {
        const label = p.displayName || p.code || "";
        personNameCache.set(personId, label);
        if (alive) setName(label);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [personId]);
  return (
    <Link href={`/persons/${personId}`} className="text-indigo-600 hover:underline">
      {name || ridTail(personId)}
    </Link>
  );
}

/** Create a (vacant) billet in a unit. POST /membership/v1/units/{unitId}/positions. */
export function CreatePosition({ unitId }: { unitId: string }) {
  const router = useRouter();
  const tr = useTg();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);

  // Live-fill the code from the title until the operator edits it (stable per-form suffix).
  const suffix = useRef(newSuffix());
  const [title, setTitle] = useState("");
  const [code, setCode] = useState("");
  const [codeTouched, setCodeTouched] = useState(false);
  const slug = slugify(title);
  const codeValue = codeTouched ? code : slug ? `${slug}-${suffix.current}` : "";

  return (
    <form
      className="card space-y-3 p-5"
      onSubmit={(e) => {
        e.preventDefault();
        setBusy(true);
        setErr(null);
        (async () => {
          try {
            await api.membership.createPosition(unitId, {
              code: codeValue.trim(),
              title: title.trim(),
            });
            setTitle("");
            setCode("");
            setCodeTouched(false);
            suffix.current = newSuffix();
            router.refresh();
          } catch (e) {
            setErr(e);
          } finally {
            setBusy(false);
          }
        })();
      }}
    >
      <h3 className="text-sm font-semibold text-slate-900"><T>Create position</T></h3>
      {err ? <ErrorBox error={err} /> : null}
      <div className="grid grid-cols-2 gap-3">
        <input
          name="code"
          required
          className="input"
          placeholder={tr("auto from title")}
          value={codeValue}
          onChange={(e) => {
            setCode(e.target.value);
            setCodeTouched(true);
          }}
        />
        <input
          name="title"
          required
          className="input"
          placeholder={tr("title (e.g. Commanding Officer)")}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </div>
      <button type="submit" className="btn-primary" disabled={busy}>
        {busy ? <T>Creating…</T> : <T>Create position</T>}
      </button>
    </form>
  );
}

/** Edit a position's title / sort order, and abolish/restore it. */
export function PositionAdmin({ position }: { position: Position }) {
  const { locale } = useLocale();
  const tr = useTg();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<unknown>(null);
  const abolished = (position.status ?? "").toLowerCase() === "abolished";

  return (
    <span className="inline-flex items-center gap-3">
      <button
        type="button"
        className="text-xs font-medium text-indigo-600 hover:underline"
        onClick={() => setOpen((o) => !o)}
      >
        <T>Edit</T>
      </button>
      {!abolished ? (
        <ActionButton
          method="POST"
          path={`/membership/v1/positions/${position.id}/abolish`}
          label="Abolish"
          confirm="Abolish this billet? (end any holder first)"
          tone="danger"
        />
      ) : null}
      {open ? (
        <form
          className="absolute z-20 mt-1 w-72 space-y-2 rounded-md border border-slate-200 bg-white p-3 shadow-lg"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            setBusy(true);
            setErr(null);
            (async () => {
              try {
                await api.membership.updatePosition(position.id, {
                  title: String(f.get("title") || "").trim() || undefined,
                  sortOrder: f.get("sortOrder") ? Number(f.get("sortOrder")) : undefined,
                });
                setOpen(false);
                router.refresh();
              } catch (e) {
                setErr(e);
              } finally {
                setBusy(false);
              }
            })();
          }}
        >
          {err ? <ErrorBox error={err} /> : null}
          <input
            name="title"
            className="input"
            placeholder={tr("title")}
            defaultValue={pickLabel(position.title, locale)}
          />
          <input
            name="sortOrder"
            type="number"
            className="input"
            placeholder={tr("sort order")}
            defaultValue={position.sortOrder ?? undefined}
          />
          <div className="flex gap-2">
            <button className="btn-primary" disabled={busy}>
              <T>Save</T>
            </button>
            <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
              <T>Cancel</T>
            </button>
          </div>
        </form>
      ) : null}
    </span>
  );
}

/** Assign a person to a vacant position. POST /membership/v1/positions/{positionId}/fill. */
export function FillPosition({ positionId }: { positionId: string }) {
  const router = useRouter();
  const tr = useTg();
  const [open, setOpen] = useState(false);
  const [personId, setPersonId] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  if (!open) {
    return (
      <button
        type="button"
        className="text-xs font-medium text-indigo-600 hover:underline"
        onClick={() => setOpen(true)}
      >
        <T>Fill</T>
      </button>
    );
  }
  return (
    <div className="flex items-center gap-2">
      <div className="w-56">
        <EntitySelect kind="person" placeholder={tr("person…")} onChange={setPersonId} allowEmpty />
      </div>
      <button
        type="button"
        className="btn-primary"
        disabled={busy || !personId}
        onClick={async () => {
          setBusy(true);
          setErr(null);
          try {
            await api.membership.fillPosition(positionId, { personId });
            setOpen(false);
            router.refresh();
          } catch (e) {
            setErr(errorMessage(e));
            setBusy(false);
          }
        }}
      >
        {busy ? "…" : <T>Assign</T>}
      </button>
      <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
        <T>Cancel</T>
      </button>
      {err && <span className="text-xs text-red-500">{err}</span>}
    </div>
  );
}
