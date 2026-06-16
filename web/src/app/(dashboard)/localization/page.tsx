import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, PageHeader, Pill, Table } from "@/components/ui";
import { Localized } from "@/components/Localized";
import { AddLocale, ToggleLocale } from "./LocaleForms";
import type { LocaleLanguage, LocaleList } from "@/lib/api/types";

export default async function LocalizationPage() {
  let list: LocaleList | null = null;
  let localeLanguages: { localeLanguages: LocaleLanguage[] } | null = null;
  let error: unknown = null;
  try {
    list = await apiGet<LocaleList>("/localization/v1/locales");
    localeLanguages = await apiGet<{ localeLanguages: LocaleLanguage[] }>(
      "/localization/v1/locale-languages",
    ).catch(() => null);
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

      <h2 className="mb-2 mt-8 text-sm font-semibold text-slate-900">Canonical languages</h2>
      <p className="mb-3 text-xs text-slate-500">
        Each locale&apos;s canonical Glottolog language (D-Languages). Read-only — reconciled by the
        language-scheme import (matching the locale&apos;s ISO 639-3 code to a languoid).
      </p>
      {localeLanguages && localeLanguages.localeLanguages.length > 0 ? (
        <Table
          head={
            <>
              <th className="th">Locale</th>
              <th className="th">Language</th>
              <th className="th">Languoid</th>
            </>
          }
        >
          {localeLanguages.localeLanguages.map((ll) => (
            <tr key={ll.locale}>
              <td className="td">
                <Mono>{ll.locale}</Mono>
              </td>
              <td className="td">
                <Localized map={ll.name} />
              </td>
              <td className="td">
                <Mono>{ll.languageId}</Mono>
              </td>
            </tr>
          ))}
        </Table>
      ) : (
        <EmptyState>No canonical languages linked yet — run the language-scheme import.</EmptyState>
      )}
    </div>
  );
}
