import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader } from "@/components/ui";
import { T } from "@/components/T";
import { DocTypeManager, SchemeManager } from "./CatalogForms";
import type { DocumentType, PersonalCodeScheme } from "@/lib/api/types";

export default async function DocumentsPage() {
  let types: DocumentType[] | null = null;
  let schemes: PersonalCodeScheme[] | null = null;
  let error: unknown = null;
  try {
    [types, schemes] = await Promise.all([
      apiGet<DocumentType[]>("/document/v1/document-types"),
      apiGet<PersonalCodeScheme[]>("/document/v1/personal-code-schemes"),
    ]);
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title={<T>Documents</T>}
        description={<T>Catalogs for person-held papers and national-identifier schemes. A person's actual documents and (encrypted) codes live on their detail page.</T>}
      />
      {error ? <ErrorNotice error={error} /> : null}

      <div className="grid gap-6 lg:grid-cols-2">
        <div>
          <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>Document types</T></h2>
          {types ? <DocTypeManager types={types} /> : <EmptyState><T>No document types.</T></EmptyState>}
        </div>

        <div>
          <h2 className="mb-3 text-sm font-semibold text-slate-900"><T>Personal-code schemes</T></h2>
          {schemes ? (
            <SchemeManager schemes={schemes} />
          ) : (
            <EmptyState><T>No personal-code schemes.</T></EmptyState>
          )}
        </div>
      </div>
    </div>
  );
}
