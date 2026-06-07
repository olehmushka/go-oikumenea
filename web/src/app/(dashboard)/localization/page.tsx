import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, PageHeader, Pill, Table } from "@/components/ui";
import { AddLocale, ToggleLocale } from "./LocaleForms";
import type { LocaleList } from "@/lib/api/types";

export default async function LocalizationPage() {
  let list: LocaleList | null = null;
  let error: unknown = null;
  try {
    list = await apiGet<LocaleList>("/localization/v1/locales");
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title="Localization"
        description="Instance-admin-managed supported locales. Every translatable label is returned in all locales — there is no Accept-Language negotiation."
      />
      {error ? <ErrorNotice error={error} /> : null}

      {list && list.locales.length > 0 ? (
        <Table
          head={
            <>
              <th className="th">Code</th>
              <th className="th">Name</th>
              <th className="th">Default</th>
              <th className="th">Enabled</th>
              <th className="th"></th>
            </>
          }
        >
          {list.locales
            .slice()
            .sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0))
            .map((l) => (
              <tr key={l.code}>
                <td className="td">
                  <Mono>{l.code}</Mono>
                </td>
                <td className="td">{l.name}</td>
                <td className="td">{l.isDefault ? <Pill tone="indigo">default</Pill> : "—"}</td>
                <td className="td">
                  <Pill tone={l.enabled ? "green" : "slate"}>
                    {l.enabled ? "enabled" : "disabled"}
                  </Pill>
                </td>
                <td className="td text-right">
                  {!l.isDefault && <ToggleLocale code={l.code} enabled={l.enabled} />}
                </td>
              </tr>
            ))}
        </Table>
      ) : (
        <EmptyState>No locales.</EmptyState>
      )}

      <div className="mt-6 max-w-md">
        <AddLocale />
      </div>
    </div>
  );
}
