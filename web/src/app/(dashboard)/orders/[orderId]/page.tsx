import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { Card, EmptyState, ErrorNotice, Mono, PageHeader, Pill } from "@/components/ui";
import { Localized } from "@/components/Localized";
import { T } from "@/components/T";
import { EditOrder, OrderActions } from "../OrderForms";
import { OrderItemsEditor } from "./OrderItemsEditor";
import { isDraft, statusTone } from "../status";
import type { Order, OrderType } from "@/lib/api/types";

/** Resolve display labels for the ids an item points at. Each lookup catches on its own, so one 403
 *  (a person the caller may not read) degrades that one label to a RID tail instead of blanking the
 *  card. Only DISTINCT ids are fetched, and the rank scheme only when some item names a rank. */
async function resolveLabels(order: Order | null) {
  const labels = new Map<string, string>();
  if (!order?.items?.length) return labels;
  const ok = await oikumenea();
  const uniq = (xs: (string | null | undefined)[]) =>
    [...new Set(xs.filter((x): x is string => !!x))];

  const persons = uniq(order.items.map((i) => i.personId));
  const units = uniq(order.items.map((i) => i.unitId));
  const positions = uniq(order.items.map((i) => i.positionId));
  const needRanks = order.items.some((i) => i.rankId);

  await Promise.all([
    ...persons.map((id) =>
      ok.person
        .getPerson(id)
        .then((p) => labels.set(id, p.displayName || p.code || id))
        .catch(() => undefined),
    ),
    ...units.map((id) =>
      ok.tenant
        .getUnit(id)
        .then((u) => labels.set(id, u.code || id))
        .catch(() => undefined),
    ),
    ...positions.map((id) =>
      ok.membership
        .getPosition(id)
        .then((p) => labels.set(id, p.code || id))
        .catch(() => undefined),
    ),
    needRanks
      ? ok.rank
          .getRankScheme()
          .then((scheme) => {
            // systems → categories → types (a TREE via `children`) → ranks
            const walk = (types: { children?: unknown[]; ranks?: unknown[] }[]) => {
              for (const ty of types) {
                for (const r of (ty.ranks ?? []) as { id?: string; code?: string }[]) {
                  if (r.id) labels.set(r.id, r.code ?? r.id);
                }
                walk((ty.children ?? []) as { children?: unknown[]; ranks?: unknown[] }[]);
              }
            };
            for (const sys of scheme.systems ?? []) {
              for (const cat of sys.categories ?? []) walk(cat.types ?? []);
            }
          })
          .catch(() => undefined)
      : Promise.resolve(),
  ]);
  return labels;
}

export default async function OrderDetailPage({
  params,
}: {
  params: Promise<{ orderId: string }>;
}) {
  const { orderId } = await params;
  let order: Order | null = null;
  let orderTypes: OrderType[] = [];
  let error: unknown = null;
  try {
    const ok = await oikumenea();
    [order, orderTypes] = await Promise.all([
      ok.order.getOrder(orderId),
      ok.order.listOrderTypes().catch(() => [] as OrderType[]),
    ]);
  } catch (e) {
    error = e;
  }

  if (error) {
    return (
      <div>
        <PageHeader title={<T>Order</T>} />
        <ErrorNotice error={error} />
      </div>
    );
  }

  const labels = await resolveLabels(order);
  const typeById = new Map(orderTypes.map((t) => [t.id, t]));
  const label = (id?: string | null) => (id ? (labels.get(id) ?? id.slice(-8)) : "");

  return (
    <div>
      <PageHeader
        title={<><T>Order</T> {order?.number ?? orderId.slice(-8)}</>}
        action={
          <Link href="/orders" className="btn-ghost">
            ← <T>Orders</T>
          </Link>
        }
      />
      <div className="grid gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900">
            <T>Items</T>{" "}
            <span className="font-normal text-slate-400">({order?.items?.length ?? 0})</span>
          </h2>
          {order?.items && order.items.length > 0 ? (
            <ul className="mt-3 space-y-2 text-sm">
              {order.items.map((it, i) => {
                const type = it.typeId ? typeById.get(it.typeId) : undefined;
                return (
                  <li key={it.id ?? i} className="rounded border border-slate-100 p-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <Pill tone="indigo">{i + 1}</Pill>
                      {type ? (
                        <span className="font-medium text-slate-800">
                          <Localized map={type.name} fallback={type.code} />{" "}
                          <span className="font-mono text-xs text-slate-400">{type.code}</span>
                        </span>
                      ) : (
                        <Mono>{it.typeId?.slice(-8)}</Mono>
                      )}
                      {type?.effect ? (
                        <span className="text-xs text-slate-400">{type.effect}</span>
                      ) : null}
                    </div>
                    <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-slate-600">
                      {it.personId ? (
                        <Link href={`/persons/${it.personId}`} className="text-indigo-600 hover:underline">
                          {label(it.personId)}
                        </Link>
                      ) : null}
                      {/* A position, when set, is what the effect acts on — the unit is then only its
                          owner (membership/application/service.go:351 branches on position first). */}
                      {it.positionId ? (
                        <span>
                          <span className="text-slate-400">⌖ </span>
                          {label(it.positionId)}
                        </span>
                      ) : it.unitId ? (
                        <Link href={`/units/${it.unitId}`} className="text-indigo-600 hover:underline">
                          {label(it.unitId)}
                        </Link>
                      ) : it.rankId ? (
                        <span>
                          <span className="text-slate-400">★ </span>
                          {label(it.rankId)}
                        </span>
                      ) : (
                        <span className="text-slate-400"><T>no target</T></span>
                      )}
                      {it.effectiveFrom || it.effectiveTo ? (
                        <span className="text-xs text-slate-500">
                          {it.effectiveFrom ?? "…"} → {it.effectiveTo ?? "…"}
                        </span>
                      ) : null}
                    </div>
                    {it.note ? <p className="mt-1 text-xs text-slate-500">{it.note}</p> : null}
                  </li>
                );
              })}
            </ul>
          ) : (
            <EmptyState><T>No items.</T></EmptyState>
          )}
          {order && isDraft(order.status) ? (
            // Remount on every successful save: router.refresh() re-renders this server component
            // with fresh items, but would NOT re-run the editor's useState initializer.
            <OrderItemsEditor
              key={order.updatedAt}
              order={order}
              orderTypes={orderTypes}
              labels={Object.fromEntries(labels)}
            />
          ) : null}
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900"><T>Status</T></h2>
          <div className="mt-3 space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-slate-500"><T>State</T></span>
              <Pill tone={statusTone(order?.status)}>{order?.status ?? "—"}</Pill>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500"><T>Issued on</T></span>
              <span>{order?.issuedOn ?? "—"}</span>
            </div>
          </div>
          <div className="mt-4">
            <OrderActions orderId={orderId} status={order?.status} />
          </div>
          {order ? <EditOrder order={order} /> : null}
          <p className="mt-3 text-xs text-slate-400">
            <T>Issuing an order applies its effects synchronously and records provenance; revoking is the legal counter-act.</T>
          </p>
        </Card>
      </div>
    </div>
  );
}
