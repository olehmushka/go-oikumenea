// The dashboard half of the explorer (M57 ticket 3, D-ConsoleDashboards): `?view=dashboard` on the
// same `/explore/[type]` URL, over the same filter state, rendered from the same facet vocabulary the
// list filters come from.
//
// A Server Component, and the charts under it are too: a segment is an <a> to this URL with one more
// filter applied, so click-to-filter is ordinary navigation and needs no client JS at all. The hover
// readout is a native <title>; the numbers are on the marks.
//
// WHAT IT ASKS FOR. One round-trip per dashboard, carrying the `facets` CSV of exactly the facets the
// charts DRAW — never every facet the type declares. At scale that is the difference between 11 s and
// 3 s (review-2026-07 § M57 ticket 2), and it is the only performance lever the console has, because
// the counts are exact by contract and will not be estimated.
//
// WHAT IT ASKS FOR TWICE, and why that is not a workaround: the age pyramid is a CROSS-TAB
// (birthdate × sex) and a per-facet stats endpoint cannot answer one, so each wing is fetched as the
// same request state plus `sex=…`. The "expiring within 90 days" tile is a bounded count over a
// window, so it is the same request plus a date range and `facets=` (count only). Both are just the
// filter state the operator could type — which is what makes their segments clickable.
//
// LOCALE. Chart titles and notes are <T> islands (ChartCard), because a cached RSC payload would
// otherwise keep a stale translation after a locale switch. Everything inside an SVG is resolved here
// on the server against getActiveLocale() — text nodes in an SVG cannot be client islands without
// making the whole chart one, and the switcher calls router.refresh() anyway.

import { ErrorNotice } from "@/components/ui";
import { BarChart } from "@/components/charts/BarChart";
import { ChartCard, NoData } from "@/components/charts/ChartCard";
import { DonutChart } from "@/components/charts/DonutChart";
import { Histogram } from "@/components/charts/Histogram";
import { PyramidChart, type PyramidRow } from "@/components/charts/PyramidChart";
import { Sparkline } from "@/components/charts/Sparkline";
import { segmentTiles, StatTile, StatTileRow } from "@/components/charts/StatTile";
import { fmtInt, fmtPct, fmtTimeBucket, TONE_FILL, type Segment } from "@/components/charts/theme";
import { getActiveLocale, pickLabel } from "@/lib/i18n";
import { tg } from "@/lib/messages";
import { oikumenea } from "@/lib/api/server";
import { bucketPatch, isoDate, plusDays } from "@/lib/ontology/buckets";
import { exploreHref, statsQuery } from "@/lib/ontology/filters";
import { OBJECT_TYPES, type ChartDef, type ObjectTypeDef, type Tone } from "@/lib/ontology/registry";
import { ridTail } from "@/lib/ontology/rid";
import {
  BUCKET_OTHER,
  BUCKET_UNKNOWN,
  distribution,
  facetsCsv,
  foldTail,
  isSyntheticBucket,
  timeSpan,
  splitUnknown,
  type StatsBucket,
  type StatsResponse,
} from "@/lib/ontology/stats";

/** How many days ahead the document "expiring soon" tile looks. */
const EXPIRY_WINDOW_DAYS = 90;

/** The `facets` CSV that asks for the total and no distributions. NOT the empty string, which the
 *  endpoint reads as "every facet the caller may read" — a list of nothing is a lone separator. */
const COUNT_ONLY = ",";

export async function Dashboard({ type, search }: { type: string; search: string }) {
  const def = OBJECT_TYPES[type];
  const dash = def?.dashboard;
  if (!def || !dash) return null;

  const sp = new URLSearchParams(search);
  const locale = getActiveLocale();
  const now = new Date();

  // Every extra request is the SAME request state plus one narrowing, so each answer is a set the
  // operator could reach by typing the filter — which is what lets its segments stay clickable.
  const extras: { id: string; query: string }[] = [];
  for (const c of dash.charts) {
    if (c.splitBy && !sp.get(c.splitBy.param)) {
      for (const v of c.splitBy.values) {
        extras.push({
          id: `${c.key}:${v}`,
          query: statsQuery(def, sp, c.facet, { [c.splitBy.param]: v }),
        });
      }
    }
    if (c.derived === "expiringSoon") {
      const f = (def.filters ?? []).find((x) => x.key === c.facet);
      if (f && f.params.length >= 2) {
        extras.push({
          id: c.key,
          // `facets=,` — the COUNT ALONE. The separator matters: an EMPTY `facets` means "every facet
          // the caller may read" (the omitted-arg default), so passing "" here quietly asked for the
          // whole dashboard a second time. A list of nothing is what asks for nothing.
          query: statsQuery(def, sp, COUNT_ONLY, {
            [f.params[0]]: isoDate(now),
            [f.params[1]]: isoDate(plusDays(now, EXPIRY_WINDOW_DAYS)),
          }),
        });
      }
    }
  }

  let main: StatsResponse;
  const side = new Map<string, StatsResponse>();
  try {
    const ok = await oikumenea();
    const get = (query: string) =>
      ok.request("GET", dash.path, { query }) as Promise<StatsResponse>;
    const [head, ...rest] = await Promise.all([
      get(statsQuery(def, sp, facetsCsv(dash.charts.map((c) => c.facet)))),
      ...extras.map((e) => get(e.query)),
    ]);
    main = head;
    extras.forEach((e, i) => side.set(e.id, rest[i]));
  } catch (e) {
    return <ErrorNotice error={e} />;
  }

  const ctx: Ctx = { def, sp, locale, now, total: main.totalCount, swatches: await swatches(dash.charts) };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-2">
        {dash.charts.map((c) => (
          <Chart key={c.key} chart={c} main={main} side={side} ctx={ctx} />
        ))}
      </div>
      <p className="text-xs text-slate-400">
        {tg("Counts are exact for what you may read:", locale)}{" "}
        {fmtInt(main.totalCount, locale)}{" "}
        {tg("rows match these filters.", locale)}
      </p>
    </div>
  );
}

// ── one chart ───────────────────────────────────────────────────────────────

type Ctx = {
  def: ObjectTypeDef;
  sp: URLSearchParams;
  locale: string;
  now: Date;
  total: number;
  /**
   * `platform_colors` RID → hex, fetched once per dashboard and only when a chart declares
   * `swatch: "color"` (M58 ticket 3 / D-Color). The stats response carries a bucket's KEY and its
   * locale→text label, never its hex — a palette entry's swatch is catalog data, not aggregate data,
   * so the one chart that paints with it resolves it here rather than widening every bucket.
   */
  swatches?: Map<string, string>;
};

function Chart({
  chart,
  main,
  side,
  ctx,
}: {
  chart: ChartDef;
  main: StatsResponse;
  side: Map<string, StatsResponse>;
  ctx: Ctx;
}) {
  const buckets = distribution(main, chart.facet);
  // ABSENT is not EMPTY: a facet whose read code the caller lacks is omitted from the response
  // (D-ObjectFacets rule 2), and drawing a zeroed chart would invent a number they may not read.
  if (buckets === undefined) return null;

  switch (chart.form) {
    case "tiles":
      return <TilesCard chart={chart} buckets={buckets} ctx={ctx} />;
    case "donut":
      return <DonutCard chart={chart} buckets={buckets} ctx={ctx} />;
    case "bar":
      return <BarCard chart={chart} buckets={buckets} ctx={ctx} />;
    case "histogram":
      return <HistogramCard chart={chart} buckets={buckets} ctx={ctx} />;
    case "pyramid":
      return <PyramidCard chart={chart} buckets={buckets} side={side} ctx={ctx} />;
    case "stat":
      return <StatCard chart={chart} buckets={buckets} side={side} ctx={ctx} />;
    default:
      return null;
  }
}

function TilesCard({ chart, buckets, ctx }: CardProps) {
  const segments = toSegments(buckets, chart, ctx);
  return (
    <ChartCard title={chart.title} note={chart.note} wide>
      <StatTileRow>
        {/* The filtered total leads: every tile beside it is a share of this number. */}
        <StatTile
          label={tg("Matching rows", ctx.locale)}
          value={fmtInt(ctx.total, ctx.locale)}
          tone="indigo"
        />
        {segmentTiles(segments, ctx.locale, chart.tone ?? {})}
      </StatTileRow>
    </ChartCard>
  );
}

function DonutCard({ chart, buckets, ctx }: CardProps) {
  const all = toSegments(buckets, chart, ctx);
  const { segments, folded } = foldTail(all, chart.maxSlices ?? 6, tg("Other", ctx.locale));
  const ring = segments.reduce((n, s) => n + s.count, 0);
  return (
    <ChartCard
      title={chart.title}
      note={chart.note}
      footer={folded > 0 ? `${tg("Smaller values folded into “other”:", ctx.locale)} ${folded}` : undefined}
    >
      <DonutChart segments={segments} locale={ctx.locale} total={ring} />
    </ChartCard>
  );
}

function BarCard({ chart, buckets, ctx }: CardProps) {
  const segments = toSegments(buckets, chart, ctx);
  return (
    <ChartCard title={chart.title} note={chart.note} footer={inertNote(segments, ctx.locale)}>
      <BarChart
        segments={segments}
        locale={ctx.locale}
        orientation={chart.orientation ?? "horizontal"}
      />
    </ChartCard>
  );
}

function HistogramCard({ chart, buckets, ctx }: CardProps) {
  // The NULL bucket is not a point in time; it is reported beside the axis (the draft backlog, the
  // documents with no expiry) rather than drawn at either end of it.
  const { rest, unknown } = splitUnknown(buckets);
  const present = new Map(rest.map((b) => [b.key, b]));
  const dense = timeSpan(rest.map((b) => b.key));
  const keys = dense.length > 0 ? dense : rest.map((b) => b.key);
  // Compare like with like: a day-grain axis needs today's DAY, a month-grain axis today's month.
  const nowKey =
    dense.length > 0 && /^\d{4}-\d{2}-\d{2}$/.test(dense[0])
      ? ctx.now.toISOString().slice(0, 10)
      : `${ctx.now.getUTCFullYear()}-${String(ctx.now.getUTCMonth() + 1).padStart(2, "0")}`;

  const segments: Segment[] = keys.map((key) => {
    const b = present.get(key) ?? { key, count: 0 };
    const seg = toSegment(b, chart, ctx);
    return chart.pastDue && key < nowKey ? { ...seg, color: TONE_FILL.red } : seg;
  });

  return (
    <ChartCard
      title={chart.title}
      note={chart.note}
      wide
      footer={
        unknown ? (
          <>
            {tg(chart.facet === "issuedOn" ? "No issue date:" : "No date recorded:", ctx.locale)}{" "}
            <span className="tabular-nums text-slate-600">{fmtInt(unknown.count, ctx.locale)}</span>
          </>
        ) : undefined
      }
    >
      <Histogram segments={segments} locale={ctx.locale} />
    </ChartCard>
  );
}

function PyramidCard({ chart, buckets, side, ctx }: CardProps & { side: Map<string, StatsResponse> }) {
  const split = chart.splitBy;
  const active = split ? ctx.sp.get(split.param) : null;
  const wings = split && !active
    ? split.values.map((v) => ({ value: v, res: side.get(`${chart.key}:${v}`) ?? null }))
    : [];

  // Already filtered to one value (or the wings failed to load): the cross-tab has collapsed, so
  // draw the honest single-series histogram of the same bands rather than a one-winged pyramid.
  if (!split || wings.length !== 2 || wings.some((w) => !w.res)) {
    const segments = toSegments(buckets, chart, ctx);
    return (
      <ChartCard title={chart.title} note={chart.note} wide footer={inertNote(segments, ctx.locale)}>
        <BarChart segments={segments} locale={ctx.locale} orientation="horizontal" />
      </ChartCard>
    );
  }

  const [left, right] = wings;
  const labelOf = (v: string) =>
    tg(enumLabel(ctx.def, split.param, v) ?? v, ctx.locale);
  const bandsOf = (res: StatsResponse) =>
    new Map((distribution(res, chart.facet) ?? []).map((b) => [b.key, b.count]));
  const l = bandsOf(left.res!);
  const r = bandsOf(right.res!);

  const rows: PyramidRow[] = buckets
    .filter((b) => !isSyntheticBucket(b.key))
    .map((b) => {
      const label = bucketLabel(b, chart, ctx);
      const wing = (value: string, counts: Map<string, number>): Segment => {
        const patch = bucketPatch(ctx.def, chart.facet, b.key, ctx.now);
        return {
          key: `${value}:${b.key}`,
          label: `${labelOf(value)} ${label}`,
          count: counts.get(b.key) ?? 0,
          href: patch
            ? exploreHref(ctx.def.type, ctx.sp, { ...patch, [split.param]: value })
            : undefined,
        };
      };
      return { key: b.key, label, left: wing(left.value, l), right: wing(right.value, r) };
    });

  const unknown = buckets.find((b) => b.key === BUCKET_UNKNOWN);
  return (
    <ChartCard
      title={chart.title}
      note={chart.note}
      wide
      footer={
        unknown ? (
          <>
            {tg("No birthdate recorded:", ctx.locale)}{" "}
            <span className="tabular-nums text-slate-600">{fmtInt(unknown.count, ctx.locale)}</span>
            {" · "}
            {tg("Sexes outside the two wings are counted in the sex chart.", ctx.locale)}
          </>
        ) : undefined
      }
    >
      <PyramidChart
        rows={rows}
        leftLabel={labelOf(left.value)}
        rightLabel={labelOf(right.value)}
        locale={ctx.locale}
      />
    </ChartCard>
  );
}

function StatCard({ chart, buckets, side, ctx }: CardProps & { side: Map<string, StatsResponse> }) {
  if (chart.derived === "revocationRate") {
    const count = (key: string) => buckets.find((b) => b.key === key)?.count ?? 0;
    const revoked = count("revoked");
    const everIssued = revoked + count("issued");
    return (
      <ChartCard title={chart.title} note={chart.note}>
        <StatTile
          label={tg("Revoked of ever-issued", ctx.locale)}
          value={fmtPct(revoked, everIssued, ctx.locale)}
          sub={`${fmtInt(revoked, ctx.locale)} / ${fmtInt(everIssued, ctx.locale)}`}
          tone={revoked > 0 ? "red" : "slate"}
        />
      </ChartCard>
    );
  }

  if (chart.derived === "expiringSoon") {
    const res = side.get(chart.key);
    if (!res) return null;
    const f = (ctx.def.filters ?? []).find((x) => x.key === chart.facet);
    const href =
      f && f.params.length >= 2
        ? exploreHref(ctx.def.type, ctx.sp, {
            [f.params[0]]: isoDate(ctx.now),
            [f.params[1]]: isoDate(plusDays(ctx.now, EXPIRY_WINDOW_DAYS)),
          })
        : undefined;
    // The trend under the number is the next twelve months of the SAME distribution the histogram
    // beside it draws, so the tile is a window on a curve rather than an isolated count.
    const ahead = buckets
      .filter((b) => !isSyntheticBucket(b.key) && b.key >= monthKey(ctx.now))
      .slice(0, 12);
    return (
      <ChartCard title={chart.title} note={chart.note}>
        <StatTile
          label={tg("Next 90 days", ctx.locale)}
          value={fmtInt(res.totalCount, ctx.locale)}
          sub={fmtPct(res.totalCount, ctx.total, ctx.locale) + " " + tg("of matching rows", ctx.locale)}
          tone={res.totalCount > 0 ? "amber" : "slate"}
          href={href}
        >
          <Sparkline
            values={ahead.map((b) => b.count)}
            title={tg("Expiries over the next twelve months", ctx.locale)}
          />
        </StatTile>
      </ChartCard>
    );
  }
  return <ChartCard title={chart.title}><NoData /></ChartCard>;
}

type CardProps = { chart: ChartDef; buckets: StatsBucket[]; ctx: Ctx };

/**
 * The `platform_colors` palette as RID → hex, for the one chart form that paints with the colour its
 * buckets NAME rather than with an encoding chosen for it (M58 ticket 3 / D-Color).
 *
 * Fetched only when some chart declares it, and failure is non-fatal: an unreachable or
 * un-permitted palette leaves the map empty and every bar falls back to the magnitude fill, which is
 * a plainer chart rather than a broken one. `hex` is optional on the catalog row too — a palette
 * entry with no swatch takes the same fallback.
 */
async function swatches(charts: ChartDef[]): Promise<Map<string, string> | undefined> {
  if (!charts.some((c) => c.swatch === "color")) return undefined;
  try {
    const ok = await oikumenea();
    const res = (await ok.request("GET", "/platform/v1/colors", {
      query: "domain=vehicle",
    })) as { colors?: { id?: string; hex?: string }[] };
    const out = new Map<string, string>();
    for (const c of res.colors ?? []) {
      if (c.id && c.hex) out.set(c.id, c.hex);
    }
    return out;
  } catch {
    return undefined;
  }
}

// ── buckets → segments ──────────────────────────────────────────────────────

function toSegments(buckets: StatsBucket[], chart: ChartDef, ctx: Ctx): Segment[] {
  return buckets.map((b) => toSegment(b, chart, ctx));
}

function toSegment(b: StatsBucket, chart: ChartDef, ctx: Ctx): Segment {
  const patch = bucketPatch(ctx.def, chart.facet, b.key, ctx.now);
  const tone: Tone | undefined = chart.tone?.[b.key];
  // A declared swatch wins over the chart's own hue, but never over a status tone: a status colour
  // means the same thing everywhere in the console and must not be repainted by data. Only a REAL
  // bucket is looked up — `(unknown)` and `(other)` name no palette row and keep the synthetic fill.
  const swatch =
    chart.swatch && !isSyntheticBucket(b.key) ? ctx.swatches?.get(b.key) : undefined;
  return {
    key: b.key,
    label: bucketLabel(b, chart, ctx),
    count: b.count,
    href: patch ? exploreHref(ctx.def.type, ctx.sp, patch) : undefined,
    color: tone ? TONE_FILL[tone] : swatch,
  };
}

/**
 * The display text of a bucket. Enum values reuse the FilterDef's own option labels — the same array
 * the filter <select> renders — so a chart segment and its filter control cannot disagree. Ref
 * buckets carry a locale→text map from the API (best effort; an unresolved id falls back to the RID
 * tail, never to nothing).
 */
function bucketLabel(b: StatsBucket, chart: ChartDef, ctx: Ctx): string {
  if (b.key === BUCKET_UNKNOWN) return tg("Unknown", ctx.locale);
  if (b.key === BUCKET_OTHER) return tg("Other", ctx.locale);

  const f = (ctx.def.filters ?? []).find((x) => x.key === chart.facet);
  if (f?.kind === "enum") return tg(enumLabel(ctx.def, f.params[0], b.key) ?? b.key, ctx.locale);
  if (f?.kind === "bool") return tg(b.key === "true" ? "Yes" : "No", ctx.locale);
  if (f?.kind === "ref") return pickLabel(b.label, ctx.locale) || ridTail(b.key);
  // A CODE bucket is shown VERBATIM: its key is its label, and it is also the filter value the
  // segment links with — `service-principal.grant` prettied into an en dash would be a label that
  // does not match the thing it filters on.
  if (f?.kind === "code") return b.key;
  if (f?.buckets === "dateTrunc") return fmtTimeBucket(b.key, ctx.locale);
  return b.key.replace("-", "–"); // a BAND reads as a range, not a subtraction
}

/** An enum option's English label, looked up by the filter's own wire param. */
function enumLabel(def: ObjectTypeDef, param: string, value: string): string | undefined {
  const f = (def.filters ?? []).find((x) => x.params[0] === param);
  return f?.values?.find((o) => o.value === value)?.label;
}

const monthKey = (d: Date) =>
  `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;

/** Says so when a chart carries segments that are not links, rather than leaving a dead mark. */
function inertNote(segments: Segment[], locale: string): React.ReactNode {
  const inert = segments.filter((s) => !s.href && s.count > 0);
  if (inert.length === 0) return undefined;
  return `${tg("Not filterable:", locale)} ${inert.map((s) => s.label).join(", ")}`;
}
