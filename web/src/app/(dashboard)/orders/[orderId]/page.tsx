import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import { Card, EmptyState, ErrorNotice, Mono, PageHeader, Pill } from "@/components/ui";
import { EditOrder, OrderActions } from "../OrderForms";
import type { Order } from "@/lib/api/types";

export default async function OrderDetailPage({
  params,
}: {
  params: Promise<{ orderId: string }>;
}) {
  const { orderId } = await params;
  let order: Order | null = null;
  let error: unknown = null;
  try {
    order = await apiGet<Order>(`/order/v1/orders/${orderId}`);
  } catch (e) {
    error = e;
  }

  if (error) {
    return (
      <div>
        <PageHeader title="Order" />
        <ErrorNotice error={error} />
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        title={`Order ${order?.number ?? orderId.slice(-8)}`}
        action={
          <Link href="/orders" className="btn-ghost">
            ← Orders
          </Link>
        }
      />
      <div className="grid gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <h2 className="text-sm font-semibold text-slate-900">Items</h2>
          {order?.items && order.items.length > 0 ? (
            <ul className="mt-3 space-y-2 text-sm">
              {order.items.map((it, i) => (
                <li key={it.id ?? i} className="rounded border border-slate-100 p-2">
                  <div className="flex gap-2">
                    <Pill tone="indigo">{it.kind ?? "item"}</Pill>
                    {it.personId && (
                      <Link
                        href={`/persons/${it.personId}`}
                        className="text-indigo-600 hover:underline"
                      >
                        <Mono>{it.personId.slice(-8)}</Mono>
                      </Link>
                    )}
                    {it.unitId && (
                      <Link
                        href={`/units/${it.unitId}`}
                        className="text-indigo-600 hover:underline"
                      >
                        <Mono>{it.unitId.slice(-8)}</Mono>
                      </Link>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          ) : (
            <EmptyState>No items.</EmptyState>
          )}
        </Card>

        <Card>
          <h2 className="text-sm font-semibold text-slate-900">Status</h2>
          <div className="mt-3 space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-slate-500">State</span>
              <Pill
                tone={
                  order?.status === "ISSUED"
                    ? "green"
                    : order?.status === "REVOKED"
                      ? "red"
                      : "slate"
                }
              >
                {order?.status ?? "—"}
              </Pill>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Issued on</span>
              <span>{order?.issuedOn ?? "—"}</span>
            </div>
          </div>
          <div className="mt-4">
            <OrderActions orderId={orderId} status={order?.status} />
          </div>
          {order ? <EditOrder order={order} /> : null}
          <p className="mt-3 text-xs text-slate-400">
            Issuing an order applies its effects synchronously and records provenance; revoking
            is the legal counter-act.
          </p>
        </Card>
      </div>
    </div>
  );
}
