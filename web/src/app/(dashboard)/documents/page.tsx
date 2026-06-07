import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, PageHeader } from "@/components/ui";
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
        title="Documents"
        description="Catalogs for person-held papers and national-identifier schemes. A person's actual documents and (encrypted) codes live on their detail page."
      />
      {error ? <ErrorNotice error={error} /> : null}

      <div className="grid gap-6 lg:grid-cols-2">
        <div>
          <h2 className="mb-3 text-sm font-semibold text-slate-900">Document types</h2>
          {types ? <DocTypeManager types={types} /> : <EmptyState>No document types.</EmptyState>}
        </div>

        <div>
          <h2 className="mb-3 text-sm font-semibold text-slate-900">Personal-code schemes</h2>
          {schemes ? (
            <SchemeManager schemes={schemes} />
          ) : (
            <EmptyState>No personal-code schemes.</EmptyState>
          )}
        </div>
      </div>
    </div>
  );
}
