import { oikumenea } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, PageHeader, Pill, Table } from "@/components/ui";
import { Localized } from "@/components/Localized";
import { T } from "@/components/T";
import { AddLocale, ToggleLocale } from "./LocaleForms";
import { TranslationEditor } from "./TranslationEditor";
import type { LocaleLanguage, LocaleList } from "@/lib/api/types";

export default async function LocalizationPage() {
  let list: LocaleList | null = null;
  let localeLanguages: { localeLanguages: LocaleLanguage[] } | null = null;
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    list = await ok.localization.listLocales();
    localeLanguages = await ok.localization.listLocaleLanguages().catch(() => null);
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title={<T>Localization</T>}
        description={<T>Instance-admin-managed supported locales. Every translatable label is returned in all locales — there is no Accept-Language negotiation.</T>}
      />
      {error ? <ErrorNotice error={error} /> : null}

      {list && list.locales.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Code</T></th>
              <th className="th"><T>Name</T></th>
              <th className="th"><T>Default</T></th>
              <th className="th"><T>Enabled</T></th>
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
                <td className="td">{l.isDefault ? <Pill tone="indigo"><T>default</T></Pill> : "—"}</td>
                <td className="td">
                  <Pill tone={l.enabled ? "green" : "slate"}>
                    {l.enabled ? <T>enabled</T> : <T>disabled</T>}
                  </Pill>
                </td>
                <td className="td text-right">
                  {!l.isDefault && <ToggleLocale code={l.code} enabled={l.enabled} />}
                </td>
              </tr>
            ))}
        </Table>
      ) : (
        <EmptyState><T>No locales.</T></EmptyState>
      )}

      <div className="mt-6 max-w-md">
        <AddLocale />
      </div>

      <div className="mt-8">
        <TranslationEditor
          locales={(list?.locales ?? [])
            .filter((l) => l.enabled)
            .sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0))
            .map((l) => ({ code: l.code, name: l.name }))}
        />
      </div>

      <h2 className="mb-2 mt-8 text-sm font-semibold text-slate-900"><T>Canonical languages</T></h2>
      <p className="mb-3 text-xs text-slate-500">
        <T>Each locale's canonical Glottolog language (D-Languages). Read-only — reconciled by the language-scheme import (matching the locale's ISO 639-3 code to a languoid).</T>
      </p>
      {localeLanguages && localeLanguages.localeLanguages.length > 0 ? (
        <Table
          head={
            <>
              <th className="th"><T>Locale</T></th>
              <th className="th"><T>Language</T></th>
              <th className="th"><T>Languoid</T></th>
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
        <EmptyState><T>No canonical languages linked yet — run the language-scheme import.</T></EmptyState>
      )}
    </div>
  );
}
