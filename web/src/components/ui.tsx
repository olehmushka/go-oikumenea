import Link from "next/link";
import { ApiError } from "@/lib/api/errors";

export function PageHeader({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="mb-5 flex items-start justify-between gap-4">
      <div>
        <h1 className="text-xl font-semibold text-slate-900">{title}</h1>
        {description && <p className="mt-1 text-sm text-slate-500">{description}</p>}
      </div>
      {action}
    </div>
  );
}

export function Card({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={`card p-5 ${className}`}>{children}</div>;
}

export function EmptyState({ children }: { children: React.ReactNode }) {
  return (
    <div className="card p-8 text-center text-sm text-slate-500">{children}</div>
  );
}

/** Renders an API failure (incl. the Conjure SerializableError envelope) readably. */
export function ErrorNotice({ error }: { error: unknown }) {
  let name = "Request failed";
  let detail = "";
  let params: Record<string, unknown> | undefined;
  if (error instanceof ApiError) {
    name = `${error.status}`;
    const b = error.body as
      | { errorName?: string; errorCode?: string; parameters?: Record<string, unknown> }
      | undefined;
    if (b?.errorName) name = b.errorName;
    if (b?.errorCode) detail = b.errorCode;
    params = b?.parameters;
  } else if (error instanceof Error) {
    detail = error.message;
  }
  return (
    <div className="card border-red-200 bg-red-50 p-5">
      <div className="text-sm font-semibold text-red-800">{name}</div>
      {detail && <div className="mt-1 text-sm text-red-700">{detail}</div>}
      {params && Object.keys(params).length > 0 && (
        <pre className="mt-2 overflow-x-auto rounded bg-red-100 p-2 text-xs text-red-900">
          {JSON.stringify(params, null, 2)}
        </pre>
      )}
      <p className="mt-3 text-xs text-red-700">
        If this is a 401, your session may have expired — try signing out and back in.
      </p>
    </div>
  );
}

export function Pill({
  children,
  tone = "slate",
}: {
  children: React.ReactNode;
  tone?: "slate" | "green" | "amber" | "red" | "indigo";
}) {
  const tones: Record<string, string> = {
    slate: "bg-slate-100 text-slate-700",
    green: "bg-green-100 text-green-800",
    amber: "bg-amber-100 text-amber-800",
    red: "bg-red-100 text-red-800",
    indigo: "bg-indigo-100 text-indigo-800",
  };
  return <span className={`badge ${tones[tone]}`}>{children}</span>;
}

export function Mono({ children }: { children: React.ReactNode }) {
  return (
    <code className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-700">
      {children}
    </code>
  );
}

/** Token pagination control: links that thread ?pageToken through the current path. */
export function Pager({
  basePath,
  nextPageToken,
  extraQuery = "",
}: {
  basePath: string;
  nextPageToken?: string | null;
  extraQuery?: string;
}) {
  if (!nextPageToken) return null;
  const sep = extraQuery ? `${extraQuery}&` : "";
  return (
    <div className="mt-4 flex justify-end">
      <Link
        href={`${basePath}?${sep}pageToken=${encodeURIComponent(nextPageToken)}`}
        className="btn-ghost"
      >
        Next page →
      </Link>
    </div>
  );
}

export function Table({
  head,
  children,
}: {
  head: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="card overflow-hidden">
      <table className="w-full">
        <thead className="border-b border-slate-200 bg-slate-50">
          <tr>{head}</tr>
        </thead>
        <tbody className="divide-y divide-slate-100">{children}</tbody>
      </table>
    </div>
  );
}
