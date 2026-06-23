import { oikumenea } from "@/lib/api/server";
import { PageHeader, ErrorNotice } from "@/components/ui";
import { T } from "@/components/T";
import { ImportsClient } from "./ImportsClient";
import type { hermenea } from "oikumenea-client";

// Always render fresh: the import control plane is live operational state, not a cacheable view.
export const dynamic = "force-dynamic";

/**
 * Imports — the hermenea ingestion control plane (M16 / D-Hermenea). All calls go to oikumenea's
 * /hermenea/v1/* routes through the normal BFF (the user's OIDC bearer); oikumenea checks import.manage
 * and re-issues to the out-of-process companion with its trigger secret. The browser never holds that
 * secret and never talks to :9443 directly.
 */
export default async function ImportsPage() {
  let sources: hermenea.IImportSource[] = [];
  let runs: hermenea.IImportRun[] = [];
  let jobs: hermenea.IWorkerJob[] = [];
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    [sources, runs, jobs] = await Promise.all([
      ok.hermenea.listSources(),
      ok.hermenea.listRuns(),
      ok.hermenea.listJobs(),
    ]);
  } catch (e) {
    error = e;
  }

  return (
    <div>
      <PageHeader
        title={<T>Imports</T>}
        description={
          <T>
            Trigger hermenea ingestion sources and watch run status live. Requires the import.manage
            permission; calls are proxied through oikumenea.
          </T>
        }
      />
      {error ? <ErrorNotice error={error} /> : null}
      <ImportsClient initialSources={sources} initialRuns={runs} initialJobs={jobs} />
    </div>
  );
}
