import { oikumenea } from "@/lib/api/server";
import { Card, ErrorNotice, Mono, PageHeader, Pill } from "@/components/ui";
import { T } from "@/components/T";

// The GDPR lawful-basis catalog (D-OverlayFoundation, M29) — Article 6 lawful bases + Article 9
// special-category conditions. Read-only reference: every future pii:special overlay store FKs a code
// here. Instance-admin edits ride PUT /platform/v1/legal-basis-kinds/{code} (not surfaced here yet).
export default async function LegalBasisPage() {
  let kinds: Array<{ code: string; name: string; article: string; status?: string; sortOrder?: number | null }> = [];
  let error: unknown = null;
  try {
    const res = await oikumenea().then((ok) => ok.platformCatalog.listLegalBasisKinds());
    kinds = res.kinds ?? [];
  } catch (e) {
    error = e;
  }

  const art6 = kinds.filter((k) => k.article === "art6");
  const art9 = kinds.filter((k) => k.article === "art9");

  return (
    <div className="max-w-3xl">
      <PageHeader
        title={<T>Legal basis catalog</T>}
        description={<T>Structured GDPR lawful bases (Art. 6) and special-category conditions (Art. 9). Referenced by every special-category overlay store.</T>}
      />
      {error ? (
        <ErrorNotice error={error} />
      ) : (
        <div className="space-y-6">
          <Section title={<T>Article 6 — lawful bases</T>} rows={art6} />
          <Section title={<T>Article 9 — special-category conditions</T>} rows={art9} />
        </div>
      )}
    </div>
  );
}

function Section({
  title,
  rows,
}: {
  title: React.ReactNode;
  rows: Array<{ code: string; name: string; status?: string }>;
}) {
  return (
    <Card>
      <h2 className="mb-3 text-sm font-semibold text-slate-900">{title}</h2>
      <ul className="divide-y divide-slate-100">
        {rows.map((k) => (
          <li key={k.code} className="flex items-center justify-between py-2">
            <div>
              <div className="text-sm font-medium text-slate-900">{k.name}</div>
              <Mono>{k.code}</Mono>
            </div>
            <Pill tone={(k.status ?? "active") === "active" ? "green" : "slate"}>{k.status ?? "active"}</Pill>
          </li>
        ))}
        {rows.length === 0 ? <li className="py-2 text-sm text-slate-500"><T>No entries.</T></li> : null}
      </ul>
    </Card>
  );
}
