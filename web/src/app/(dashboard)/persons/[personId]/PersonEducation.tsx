"use client";

// PersonEducationManager — manage all person↔education relationships (M20/M21, D-Education) from the
// person object view. The education backend exposes 8 person-path link types under /education/v1 plus a
// read-only view of the teaching/admin appointments a person holds. Layout: one card with a type
// switcher (mirrors the education page's ReferencePanel); each type owns its fetch + list + upsert form,
// following the PersonLanguageManager pattern (self-fetch via api.request, run() that writes then reloads).
//
// Most links target an institution-scoped entity, so the forms pick an institution (EntitySelect) and
// then cascade its child list (units / groups / buildings / research-groups / grants / governance-bodies
// / qualifications) into a plain <select>. Publications and scholarships are globally scoped pickers.

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { EntitySelect } from "@/components/EntitySelect";
import { ErrorBox } from "@/components/ErrorBox";
import { T } from "@/components/T";
import { pickLabel, type LocaleMap } from "@/lib/i18n";
import { useLocale, useTg } from "@/lib/locale";

/* ------------------------------------------------------------------ row types (display) */

type Enrollment = {
  id: string; institutionId: string; unitId?: string; groupId?: string; programId?: string;
  degreeLevelId?: string; fieldOfStudy?: string; studentNumber?: string; status: string;
  qualification?: string; effectiveFrom?: string; effectiveTo?: string;
};
type DormitoryStay = {
  id: string; buildingId: string; room?: string; status: string; effectiveFrom?: string; effectiveTo?: string;
};
type PublicationAuthorship = {
  id: string; publicationId: string; authorOrder?: number; corresponding?: boolean; effectiveFrom?: string; effectiveTo?: string;
};
// research / grant / governance memberships share a shape handled generically by RoleLinkSection.
type QualificationAward = {
  id: string; qualificationId: string; enrollmentId?: string; awardedOn?: string; withDistinction?: boolean; gpa?: string; status: string;
};
type ScholarshipAward = {
  id: string; scholarshipId: string; status: string; effectiveFrom?: string; effectiveTo?: string;
};
type PersonAppointment = {
  id: string; positionId: string; positionTitle: string; institutionId: string; institutionName: string;
  status: string; effectiveFrom?: string; effectiveTo?: string;
};

/* ------------------------------------------------------------------ small helpers */

const EDU = "/education/v1";
const sv = (v: unknown): string => (typeof v === "string" ? v : "");
const opt = (v: string): string | undefined => (v.trim() === "" ? undefined : v.trim());
const tail = (id: string): string => id.slice(-8);

function lbl(v: unknown, locale: string): string {
  if (typeof v === "string") return v;
  if (v && typeof v === "object") return pickLabel(v as LocaleMap, locale);
  return "";
}

// useLinks fetches a person sub-resource list and exposes a write→reload runner (PersonLanguageManager
// pattern). `key` is the list-wrapper field (enrollments / awards / memberships / …).
function useLinks<T>(path: string, key: string) {
  const [rows, setRows] = useState<T[] | null>(null);
  const [err, setErr] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(() => {
    api.request<Record<string, T[]>>("GET", path)
      .then((r) => setRows(r[key] ?? []))
      .catch(setErr);
  }, [path, key]);
  useEffect(() => {
    reload();
  }, [reload]);

  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      after?.();
      reload();
    } catch (e) {
      setErr(e);
    } finally {
      setBusy(false);
    }
  };
  return { rows, err, busy, run };
}

// ScopedSelect loads an institution-scoped (or unit-scoped) child list into a dropdown. `path` is the
// full list endpoint or "" to disable (no parent chosen yet); `listKey` is the response wrapper field.
function ScopedSelect({
  path, listKey, value, onChange, placeholder, required, filter,
}: {
  path: string;
  listKey: string;
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  required?: boolean;
  filter?: (o: Record<string, unknown>) => boolean;
}) {
  const { locale } = useLocale();
  const tr = useTg();
  const [opts, setOpts] = useState<Record<string, unknown>[]>([]);
  useEffect(() => {
    if (!path) {
      setOpts([]);
      return;
    }
    let alive = true;
    api.request<Record<string, Record<string, unknown>[]>>("GET", path)
      .then((r) => {
        if (alive) setOpts(r[listKey] ?? []);
      })
      .catch(() => {
        if (alive) setOpts([]);
      });
    return () => {
      alive = false;
    };
  }, [path, listKey]);

  const shown = filter ? opts.filter(filter) : opts;
  // When editing an institution-scoped link the row carries only the target id (not its institution), so
  // its options may not be loaded yet — surface the current value as a fallback so it stays selected.
  const missing = value && !shown.some((o) => sv(o.id) === value);
  return (
    <select
      className="input"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      required={required}
      disabled={!path}
    >
      <option value="">{path ? `${tr(placeholder)}…` : tr("pick institution first")}</option>
      {missing ? <option value={value}>{tail(value)} {tr("(current)")}</option> : null}
      {shown.map((o) => (
        <option key={sv(o.id)} value={sv(o.id)}>
          {lbl(o.name, locale) || lbl(o.title, locale) || sv(o.code) || tail(sv(o.id))}
        </option>
      ))}
    </select>
  );
}

// RowLine renders a list item with Edit + Remove actions.
function RowLine({
  children, busy, onEdit, onRemove,
}: {
  children: React.ReactNode;
  busy: boolean;
  onEdit: () => void;
  onRemove: () => void;
}) {
  return (
    <li className="flex items-center justify-between gap-2">
      <span className="min-w-0 truncate">{children}</span>
      <span className="flex shrink-0 gap-2">
        <button type="button" className="text-xs font-medium text-indigo-600 hover:underline disabled:opacity-50" disabled={busy} onClick={onEdit}>
          <T>Edit</T>
        </button>
        <button type="button" className="text-xs font-medium text-red-600 hover:underline disabled:opacity-50" disabled={busy} onClick={onRemove}>
          <T>Remove</T>
        </button>
      </span>
    </li>
  );
}

function Empty({ rows }: { rows: unknown[] | null }) {
  return rows && rows.length === 0 ? <p className="mt-1 text-sm text-slate-400">—</p> : null;
}

const listCls = "mt-1 space-y-0.5 text-sm text-slate-700";

function dates(from?: string, to?: string): string {
  if (!from && !to) return "";
  return ` · ${from ?? "?"}→${to ?? "now"}`;
}

const formCls = "mt-3 grid grid-cols-1 gap-2 border-t border-slate-100 pt-3 sm:grid-cols-2";

/* ------------------------------------------------------------------ enrollments (studied_at) */

function EnrollmentSection({ personId }: { personId: string }) {
  const base = `${EDU}/persons/${personId}/enrollments`;
  const { rows, err, busy, run } = useLinks<Enrollment>(base, "enrollments");
  const [editId, setEditId] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);
  const [inst, setInst] = useState("");
  const [unit, setUnit] = useState("");
  const [group, setGroup] = useState("");
  const [program, setProgram] = useState("");
  const [degree, setDegree] = useState("");
  const [field, setField] = useState("");
  const [student, setStudent] = useState("");
  const [status, setStatus] = useState("");
  const [qual, setQual] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [degrees, setDegrees] = useState<Record<string, unknown>[]>([]);
  const { locale } = useLocale();
  const tr = useTg();

  useEffect(() => {
    api.request<{ degreeLevels: Record<string, unknown>[] }>("GET", `${EDU}/degree-levels`)
      .then((r) => setDegrees(r.degreeLevels ?? []))
      .catch(() => {});
  }, []);

  function reset() {
    setEditId(null); setInst(""); setUnit(""); setGroup(""); setProgram(""); setDegree("");
    setField(""); setStudent(""); setStatus(""); setQual(""); setFrom(""); setTo("");
    setFormKey((k) => k + 1);
  }
  function edit(e: Enrollment) {
    setEditId(e.id); setInst(e.institutionId); setUnit(e.unitId ?? ""); setGroup(e.groupId ?? "");
    setProgram(e.programId ?? ""); setDegree(e.degreeLevelId ?? ""); setField(e.fieldOfStudy ?? "");
    setStudent(e.studentNumber ?? ""); setStatus(e.status ?? ""); setQual(e.qualification ?? "");
    setFrom(e.effectiveFrom ?? ""); setTo(e.effectiveTo ?? ""); setFormKey((k) => k + 1);
  }
  function submit(ev: React.FormEvent) {
    ev.preventDefault();
    if (!inst) return;
    const body = {
      institutionId: inst, unitId: opt(unit), groupId: opt(group), programId: opt(program),
      degreeLevelId: opt(degree), fieldOfStudy: opt(field), studentNumber: opt(student),
      status: opt(status), qualification: opt(qual), effectiveFrom: opt(from), effectiveTo: opt(to),
    };
    run(() => api.request(editId ? "PUT" : "POST", editId ? `${base}/${editId}` : base, { body }), reset);
  }

  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      <Empty rows={rows} />
      <ul className={listCls}>
        {(rows ?? []).map((e) => (
          <RowLine key={e.id} busy={busy} onEdit={() => edit(e)}
            onRemove={() => window.confirm("Remove this enrollment?") && run(() => api.request("DELETE", `${base}/${e.id}`))}>
            {e.fieldOfStudy || tail(e.institutionId)} · {e.status}
            {e.qualification ? ` · ${e.qualification}` : ""}{dates(e.effectiveFrom, e.effectiveTo)}
          </RowLine>
        ))}
      </ul>
      <form key={formKey} className={formCls} onSubmit={submit}>
        <EntitySelect kind="institution" defaultValue={inst} onChange={setInst} placeholder="Institution *" />
        <ScopedSelect path={inst ? `${EDU}/institutions/${inst}/units` : ""} listKey="units" value={unit} onChange={(v) => { setUnit(v); setGroup(""); }} placeholder="Unit" />
        <ScopedSelect path={unit ? `${EDU}/units/${unit}/groups` : ""} listKey="groups" value={group} onChange={setGroup} placeholder="Group" />
        <ScopedSelect path={inst ? `${EDU}/institutions/${inst}/programs` : ""} listKey="programs" value={program} onChange={setProgram} placeholder="Program" />
        <select className="input" value={degree} onChange={(e) => setDegree(e.target.value)}>
          <option value="">{tr("Degree level…")}</option>
          {degrees.map((d) => (
            <option key={sv(d.id)} value={sv(d.id)}>{lbl(d.name, locale) || sv(d.code)}</option>
          ))}
        </select>
        <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{tr("Status…")}</option>
          {["enrolled", "graduated", "withdrawn", "expelled", "on_leave"].map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <input className="input" placeholder={tr("Field of study")} value={field} onChange={(e) => setField(e.target.value)} />
        <input className="input" placeholder={tr("Student number")} value={student} onChange={(e) => setStudent(e.target.value)} />
        <input className="input" placeholder={tr("Qualification awarded")} value={qual} onChange={(e) => setQual(e.target.value)} />
        <div className="grid grid-cols-2 gap-2">
          <input className="input" type="date" value={from} onChange={(e) => setFrom(e.target.value)} title="from" />
          <input className="input" type="date" value={to} onChange={(e) => setTo(e.target.value)} title="to" />
        </div>
        <div className="flex gap-2 sm:col-span-2">
          <button className="btn" disabled={busy || !inst}>{editId ? <T>Save enrollment</T> : <T>Add enrollment</T>}</button>
          {editId ? <button type="button" className="btn-ghost" onClick={reset}><T>Cancel</T></button> : null}
        </div>
      </form>
    </div>
  );
}

/* ------------------------------------------------------------------ dormitory stays */

function DormitorySection({ personId }: { personId: string }) {
  const tr = useTg();
  const base = `${EDU}/persons/${personId}/dormitory-stays`;
  const { rows, err, busy, run } = useLinks<DormitoryStay>(base, "dormitoryStays");
  const [editId, setEditId] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);
  const [inst, setInst] = useState("");
  const [building, setBuilding] = useState("");
  const [room, setRoom] = useState("");
  const [status, setStatus] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  function reset() {
    setEditId(null); setInst(""); setBuilding(""); setRoom(""); setStatus(""); setFrom(""); setTo("");
    setFormKey((k) => k + 1);
  }
  function edit(d: DormitoryStay) {
    setEditId(d.id); setBuilding(d.buildingId); setRoom(d.room ?? ""); setStatus(d.status ?? "");
    setFrom(d.effectiveFrom ?? ""); setTo(d.effectiveTo ?? ""); setFormKey((k) => k + 1);
  }
  function submit(ev: React.FormEvent) {
    ev.preventDefault();
    if (!building) return;
    const body = { buildingId: building, room: opt(room), status: opt(status), effectiveFrom: opt(from), effectiveTo: opt(to) };
    run(() => api.request(editId ? "PUT" : "POST", editId ? `${base}/${editId}` : base, { body }), reset);
  }

  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      <Empty rows={rows} />
      <ul className={listCls}>
        {(rows ?? []).map((d) => (
          <RowLine key={d.id} busy={busy} onEdit={() => edit(d)}
            onRemove={() => window.confirm("Remove this dormitory stay?") && run(() => api.request("DELETE", `${base}/${d.id}`))}>
            {tail(d.buildingId)}{d.room ? ` · room ${d.room}` : ""} · {d.status}{dates(d.effectiveFrom, d.effectiveTo)}
          </RowLine>
        ))}
      </ul>
      <form key={formKey} className={formCls} onSubmit={submit}>
        <EntitySelect kind="institution" defaultValue={inst} onChange={(v) => { setInst(v); setBuilding(""); }} placeholder="Institution" />
        <ScopedSelect path={inst ? `${EDU}/institutions/${inst}/buildings` : ""} listKey="buildings" value={building} onChange={setBuilding}
          placeholder="Dormitory" required filter={(o) => o.kind === "dormitory"} />
        <input className="input" placeholder={tr("Room")} value={room} onChange={(e) => setRoom(e.target.value)} />
        <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{tr("Status…")}</option>
          {["active", "ended"].map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <div className="grid grid-cols-2 gap-2">
          <input className="input" type="date" value={from} onChange={(e) => setFrom(e.target.value)} title="from" />
          <input className="input" type="date" value={to} onChange={(e) => setTo(e.target.value)} title="to" />
        </div>
        <div className="flex gap-2 sm:col-span-2">
          <button className="btn" disabled={busy || !building}>{editId ? <T>Save stay</T> : <T>Add stay</T>}</button>
          {editId ? <button type="button" className="btn-ghost" onClick={reset}><T>Cancel</T></button> : null}
        </div>
      </form>
    </div>
  );
}

/* ------------------------------------------------------------------ publication authorships */

function PublicationSection({ personId }: { personId: string }) {
  const tr = useTg();
  const base = `${EDU}/persons/${personId}/publication-authorships`;
  const { rows, err, busy, run } = useLinks<PublicationAuthorship>(base, "authorships");
  const [editId, setEditId] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);
  const [pub, setPub] = useState("");
  const [order, setOrder] = useState("");
  const [corresponding, setCorresponding] = useState(false);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  function reset() {
    setEditId(null); setPub(""); setOrder(""); setCorresponding(false); setFrom(""); setTo("");
    setFormKey((k) => k + 1);
  }
  function edit(a: PublicationAuthorship) {
    setEditId(a.id); setPub(a.publicationId); setOrder(a.authorOrder != null ? String(a.authorOrder) : "");
    setCorresponding(!!a.corresponding); setFrom(a.effectiveFrom ?? ""); setTo(a.effectiveTo ?? ""); setFormKey((k) => k + 1);
  }
  function submit(ev: React.FormEvent) {
    ev.preventDefault();
    if (!pub) return;
    const body = {
      publicationId: pub, authorOrder: order.trim() === "" ? undefined : Number(order),
      corresponding, effectiveFrom: opt(from), effectiveTo: opt(to),
    };
    run(() => api.request(editId ? "PUT" : "POST", editId ? `${base}/${editId}` : base, { body }), reset);
  }

  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      <Empty rows={rows} />
      <ul className={listCls}>
        {(rows ?? []).map((a) => (
          <RowLine key={a.id} busy={busy} onEdit={() => edit(a)}
            onRemove={() => window.confirm("Remove this authorship?") && run(() => api.request("DELETE", `${base}/${a.id}`))}>
            {tail(a.publicationId)}{a.authorOrder != null ? ` · #${a.authorOrder}` : ""}{a.corresponding ? " · corresponding" : ""}
          </RowLine>
        ))}
      </ul>
      <form key={formKey} className={formCls} onSubmit={submit}>
        <EntitySelect kind="publication" defaultValue={pub} onChange={setPub} placeholder="Publication *" />
        <input className="input" type="number" placeholder={tr("Author order")} value={order} onChange={(e) => setOrder(e.target.value)} />
        <label className="flex items-center gap-2 text-sm text-slate-600">
          <input type="checkbox" checked={corresponding} onChange={(e) => setCorresponding(e.target.checked)} /> {tr("corresponding author")}
        </label>
        <div className="grid grid-cols-2 gap-2">
          <input className="input" type="date" value={from} onChange={(e) => setFrom(e.target.value)} title="from" />
          <input className="input" type="date" value={to} onChange={(e) => setTo(e.target.value)} title="to" />
        </div>
        <div className="flex gap-2 sm:col-span-2">
          <button className="btn" disabled={busy || !pub}>{editId ? <T>Save authorship</T> : <T>Add authorship</T>}</button>
          {editId ? <button type="button" className="btn-ghost" onClick={reset}><T>Cancel</T></button> : null}
        </div>
      </form>
    </div>
  );
}

/* --------------------------------------------- generic institution→entity role link (research/grant/governance) */

// RoleLinkSection covers the three "person holds a role in an institution-scoped body" links that share
// the shape { <idField>, role|roleInBody, status, effectiveFrom/To }.
function RoleLinkSection({
  personId, type, listKey, idField, childPath, childKey, pickLabel: pickPlaceholder, roleField, roleOptions, removeMsg, addLabel,
}: {
  personId: string;
  type: string;            // path segment, e.g. research-memberships
  listKey: string;         // response wrapper key
  idField: string;         // body field for the target id, e.g. groupId
  childPath: string;       // institution-scoped list segment, e.g. research-groups
  childKey: string;        // child list wrapper key
  pickLabel: string;       // placeholder for the target select
  roleField: string;       // role | roleInBody
  roleOptions?: string[];  // enum for role (grants); undefined → free text
  removeMsg: string;
  addLabel: string;
}) {
  const tr = useTg();
  const base = `${EDU}/persons/${personId}/${type}`;
  type Row = { id: string; role?: string; roleInBody?: string; status: string; effectiveFrom?: string; effectiveTo?: string } & Record<string, string | undefined>;
  const { rows, err, busy, run } = useLinks<Row>(base, listKey);
  const [editId, setEditId] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);
  const [inst, setInst] = useState("");
  const [target, setTarget] = useState("");
  const [role, setRole] = useState("");
  const [status, setStatus] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  function reset() {
    setEditId(null); setInst(""); setTarget(""); setRole(""); setStatus(""); setFrom(""); setTo("");
    setFormKey((k) => k + 1);
  }
  function edit(r: Row) {
    setEditId(r.id); setTarget(sv(r[idField])); setRole(sv(r[roleField])); setStatus(r.status ?? "");
    setFrom(r.effectiveFrom ?? ""); setTo(r.effectiveTo ?? ""); setFormKey((k) => k + 1);
  }
  function submit(ev: React.FormEvent) {
    ev.preventDefault();
    if (!target) return;
    const body: Record<string, unknown> = {
      [idField]: target, [roleField]: opt(role), status: opt(status), effectiveFrom: opt(from), effectiveTo: opt(to),
    };
    run(() => api.request(editId ? "PUT" : "POST", editId ? `${base}/${editId}` : base, { body }), reset);
  }

  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      <Empty rows={rows} />
      <ul className={listCls}>
        {(rows ?? []).map((r) => (
          <RowLine key={r.id} busy={busy} onEdit={() => edit(r)}
            onRemove={() => window.confirm(removeMsg) && run(() => api.request("DELETE", `${base}/${r.id}`))}>
            {tail(sv(r[idField]))}{sv(r[roleField]) ? ` · ${sv(r[roleField])}` : ""} · {r.status}{dates(r.effectiveFrom, r.effectiveTo)}
          </RowLine>
        ))}
      </ul>
      <form key={formKey} className={formCls} onSubmit={submit}>
        <EntitySelect kind="institution" defaultValue={inst} onChange={(v) => { setInst(v); setTarget(""); }} placeholder="Institution" />
        <ScopedSelect path={inst ? `${EDU}/institutions/${inst}/${childPath}` : ""} listKey={childKey} value={target} onChange={setTarget} placeholder={pickPlaceholder} required />
        {roleOptions ? (
          <select className="input" value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="">{tr("Role…")}</option>
            {roleOptions.map((o) => <option key={o} value={o}>{o}</option>)}
          </select>
        ) : (
          <input className="input" placeholder={tr("Role")} value={role} onChange={(e) => setRole(e.target.value)} />
        )}
        <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{tr("Status…")}</option>
          {["active", "ended"].map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <div className="grid grid-cols-2 gap-2">
          <input className="input" type="date" value={from} onChange={(e) => setFrom(e.target.value)} title="from" />
          <input className="input" type="date" value={to} onChange={(e) => setTo(e.target.value)} title="to" />
        </div>
        <div className="flex gap-2 sm:col-span-2">
          <button className="btn" disabled={busy || !target}>{editId ? <T>Save</T> : tr(addLabel)}</button>
          {editId ? <button type="button" className="btn-ghost" onClick={reset}><T>Cancel</T></button> : null}
        </div>
      </form>
    </div>
  );
}

/* ------------------------------------------------------------------ qualification awards */

function QualificationSection({ personId }: { personId: string }) {
  const tr = useTg();
  const base = `${EDU}/persons/${personId}/qualification-awards`;
  const { rows, err, busy, run } = useLinks<QualificationAward>(base, "awards");
  const [editId, setEditId] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);
  const [inst, setInst] = useState("");
  const [qualId, setQualId] = useState("");
  const [enrollment, setEnrollment] = useState("");
  const [awardedOn, setAwardedOn] = useState("");
  const [distinction, setDistinction] = useState(false);
  const [gpa, setGpa] = useState("");
  const [status, setStatus] = useState("");
  const [enrollments, setEnrollments] = useState<Enrollment[]>([]);

  useEffect(() => {
    api.request<{ enrollments: Enrollment[] }>("GET", `${EDU}/persons/${personId}/enrollments`)
      .then((r) => setEnrollments(r.enrollments ?? []))
      .catch(() => {});
  }, [personId]);

  function reset() {
    setEditId(null); setInst(""); setQualId(""); setEnrollment(""); setAwardedOn(""); setDistinction(false);
    setGpa(""); setStatus(""); setFormKey((k) => k + 1);
  }
  function edit(a: QualificationAward) {
    setEditId(a.id); setQualId(a.qualificationId); setEnrollment(a.enrollmentId ?? ""); setAwardedOn(a.awardedOn ?? "");
    setDistinction(!!a.withDistinction); setGpa(a.gpa ?? ""); setStatus(a.status ?? ""); setFormKey((k) => k + 1);
  }
  function submit(ev: React.FormEvent) {
    ev.preventDefault();
    if (!qualId) return;
    const body = {
      qualificationId: qualId, enrollmentId: opt(enrollment), awardedOn: opt(awardedOn),
      withDistinction: distinction, gpa: opt(gpa), status: opt(status),
    };
    run(() => api.request(editId ? "PUT" : "POST", editId ? `${base}/${editId}` : base, { body }), reset);
  }

  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      <Empty rows={rows} />
      <ul className={listCls}>
        {(rows ?? []).map((a) => (
          <RowLine key={a.id} busy={busy} onEdit={() => edit(a)}
            onRemove={() => window.confirm("Remove this qualification award?") && run(() => api.request("DELETE", `${base}/${a.id}`))}>
            {tail(a.qualificationId)} · {a.status}{a.withDistinction ? " · distinction" : ""}{a.awardedOn ? ` · ${a.awardedOn}` : ""}
          </RowLine>
        ))}
      </ul>
      <form key={formKey} className={formCls} onSubmit={submit}>
        <EntitySelect kind="institution" defaultValue={inst} onChange={(v) => { setInst(v); setQualId(""); }} placeholder="Institution" />
        <ScopedSelect path={inst ? `${EDU}/institutions/${inst}/qualifications` : ""} listKey="qualifications" value={qualId} onChange={setQualId} placeholder="Qualification" required />
        <select className="input" value={enrollment} onChange={(e) => setEnrollment(e.target.value)}>
          <option value="">{tr("Link enrollment…")}</option>
          {enrollments.map((e) => <option key={e.id} value={e.id}>{e.fieldOfStudy || tail(e.institutionId)}</option>)}
        </select>
        <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{tr("Status…")}</option>
          {["awarded", "revoked"].map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <input className="input" type="date" value={awardedOn} onChange={(e) => setAwardedOn(e.target.value)} title="awarded on" />
        <input className="input" placeholder={tr("GPA")} value={gpa} onChange={(e) => setGpa(e.target.value)} />
        <label className="flex items-center gap-2 text-sm text-slate-600">
          <input type="checkbox" checked={distinction} onChange={(e) => setDistinction(e.target.checked)} /> {tr("with distinction")}
        </label>
        <div className="flex gap-2 sm:col-span-2">
          <button className="btn" disabled={busy || !qualId}>{editId ? <T>Save award</T> : <T>Add award</T>}</button>
          {editId ? <button type="button" className="btn-ghost" onClick={reset}><T>Cancel</T></button> : null}
        </div>
      </form>
    </div>
  );
}

/* ------------------------------------------------------------------ scholarship awards */

function ScholarshipSection({ personId }: { personId: string }) {
  const tr = useTg();
  const base = `${EDU}/persons/${personId}/scholarship-awards`;
  const { rows, err, busy, run } = useLinks<ScholarshipAward>(base, "awards");
  const [editId, setEditId] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);
  const [scholarship, setScholarship] = useState("");
  const [status, setStatus] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  function reset() {
    setEditId(null); setScholarship(""); setStatus(""); setFrom(""); setTo(""); setFormKey((k) => k + 1);
  }
  function edit(a: ScholarshipAward) {
    setEditId(a.id); setScholarship(a.scholarshipId); setStatus(a.status ?? "");
    setFrom(a.effectiveFrom ?? ""); setTo(a.effectiveTo ?? ""); setFormKey((k) => k + 1);
  }
  function submit(ev: React.FormEvent) {
    ev.preventDefault();
    if (!scholarship) return;
    const body = { scholarshipId: scholarship, status: opt(status), effectiveFrom: opt(from), effectiveTo: opt(to) };
    run(() => api.request(editId ? "PUT" : "POST", editId ? `${base}/${editId}` : base, { body }), reset);
  }

  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      <Empty rows={rows} />
      <ul className={listCls}>
        {(rows ?? []).map((a) => (
          <RowLine key={a.id} busy={busy} onEdit={() => edit(a)}
            onRemove={() => window.confirm("Remove this scholarship award?") && run(() => api.request("DELETE", `${base}/${a.id}`))}>
            {tail(a.scholarshipId)} · {a.status}{dates(a.effectiveFrom, a.effectiveTo)}
          </RowLine>
        ))}
      </ul>
      <form key={formKey} className={formCls} onSubmit={submit}>
        <EntitySelect kind="scholarship" defaultValue={scholarship} onChange={setScholarship} placeholder="Scholarship *" />
        <select className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{tr("Status…")}</option>
          {["active", "suspended", "terminated", "completed"].map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
        <div className="grid grid-cols-2 gap-2">
          <input className="input" type="date" value={from} onChange={(e) => setFrom(e.target.value)} title="from" />
          <input className="input" type="date" value={to} onChange={(e) => setTo(e.target.value)} title="to" />
        </div>
        <div className="flex gap-2 sm:col-span-2">
          <button className="btn" disabled={busy || !scholarship}>{editId ? <T>Save award</T> : <T>Add award</T>}</button>
          {editId ? <button type="button" className="btn-ghost" onClick={reset}><T>Cancel</T></button> : null}
        </div>
      </form>
    </div>
  );
}

/* ------------------------------------------------------------------ appointments (read-only) */

function AppointmentSection({ personId }: { personId: string }) {
  const { rows, err } = useLinks<PersonAppointment>(`${EDU}/persons/${personId}/appointments`, "appointments");
  return (
    <div>
      {err ? <div className="mb-2"><ErrorBox error={err} /></div> : null}
      <Empty rows={rows} />
      <ul className={listCls}>
        {(rows ?? []).map((a) => (
          <li key={a.id} className="flex items-center justify-between gap-2">
            <span className="min-w-0 truncate">
              {a.positionTitle} · {a.institutionName} · {a.status}{dates(a.effectiveFrom, a.effectiveTo)}
            </span>
          </li>
        ))}
      </ul>
      <p className="mt-2 text-xs text-slate-400">
        <T>Read-only. Teaching/admin positions are filled and ended from the Education institution view.</T>
      </p>
    </div>
  );
}

/* ------------------------------------------------------------------ switcher */

const TABS = [
  { key: "enrollments", label: "Enrollments" },
  { key: "dormitory", label: "Dormitory stays" },
  { key: "publications", label: "Publication authorships" },
  { key: "research", label: "Research memberships" },
  { key: "grants", label: "Grant holdings" },
  { key: "governance", label: "Governance memberships" },
  { key: "qualifications", label: "Qualification awards" },
  { key: "scholarships", label: "Scholarship awards" },
  { key: "appointments", label: "Appointments (read-only)" },
] as const;

type TabKey = (typeof TABS)[number]["key"];

export function PersonEducationManager({ personId }: { personId: string }) {
  const tr = useTg();
  const [tab, setTab] = useState<TabKey>("enrollments");
  return (
    <div>
      <div className="mb-3 flex items-center justify-between gap-2">
        <p className="text-xs font-medium uppercase tracking-wide text-slate-400"><T>Education relationships</T></p>
        <select className="input w-56" value={tab} onChange={(e) => setTab(e.target.value as TabKey)}>
          {TABS.map((t) => <option key={t.key} value={t.key}>{tr(t.label)}</option>)}
        </select>
      </div>
      {tab === "enrollments" && <EnrollmentSection personId={personId} />}
      {tab === "dormitory" && <DormitorySection personId={personId} />}
      {tab === "publications" && <PublicationSection personId={personId} />}
      {tab === "research" && (
        <RoleLinkSection personId={personId} type="research-memberships" listKey="memberships" idField="groupId"
          childPath="research-groups" childKey="researchGroups" pickLabel="Research group" roleField="role"
          removeMsg="Remove this research membership?" addLabel="Add membership" />
      )}
      {tab === "grants" && (
        <RoleLinkSection personId={personId} type="grant-holdings" listKey="holdings" idField="grantId"
          childPath="grants" childKey="grants" pickLabel="Grant" roleField="role"
          roleOptions={["pi", "co_investigator", "researcher", "administrator"]}
          removeMsg="Remove this grant holding?" addLabel="Add holding" />
      )}
      {tab === "governance" && (
        <RoleLinkSection personId={personId} type="governance-memberships" listKey="memberships" idField="bodyId"
          childPath="governance-bodies" childKey="governanceBodies" pickLabel="Governance body" roleField="roleInBody"
          removeMsg="Remove this governance membership?" addLabel="Add membership" />
      )}
      {tab === "qualifications" && <QualificationSection personId={personId} />}
      {tab === "scholarships" && <ScholarshipSection personId={personId} />}
      {tab === "appointments" && <AppointmentSection personId={personId} />}
    </div>
  );
}
