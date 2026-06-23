import Link from "next/link";
import { oikumenea } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, Pager, PageHeader, Pill, Table } from "@/components/ui";
import { LookupForm } from "@/components/LookupForm";
import { T } from "@/components/T";
import { OrderCreate, OrderTypeManager } from "./OrderForms";
import type { OrderPage, OrderType } from "@/lib/api/types";

export default async function OrdersPage({
  searchParams,
}: {
  searchParams: Promise<{ unitId?: string; pageToken?: string }>;
}) {
  const { unitId, pageToken } = await searchParams;
  let page: OrderPage | null = null;
  let error: unknown = null;
  const ok = await oikumenea();
  const orderTypes = await ok.order.listOrderTypes().catch(() => []);
  if (unitId) {
    try {
      page = await ok.order.listUnitOrders(unitId, 50, pageToken ?? undefined);
    } catch (e) {
      error = e;
    }
  }

  return (
    <div>
      <PageHeader
        title={<T>Orders</T>}
        description={<T>Administrative orders (наказ) — the legal basis for status changes. Effects apply on issue, with provenance.</T>}
      />

      <div className="mb-5 max-w-md">
        <LookupForm
          basePath="/orders"
          param="unitId"
          label="Issuing unit"
          kind="unit"
          current={unitId}
        />
      </div>

      {error ? <ErrorNotice error={error} /> : null}

      {unitId && page && (
        <>
          {page.orders.length > 0 ? (
            <Table
              head={
                <>
                  <th className="th"><T>Number</T></th>
                  <th className="th"><T>Issued on</T></th>
                  <th className="th"><T>Items</T></th>
                  <th className="th"><T>Status</T></th>
                </>
              }
            >
              {page.orders.map((o) => (
                <tr key={o.id}>
                  <td className="td">
                    <Link
                      href={`/orders/${o.id}`}
                      className="text-indigo-600 hover:underline"
                    >
                      <Mono>{o.number ?? o.id.slice(-8)}</Mono>
                    </Link>
                  </td>
                  <td className="td">{o.issuedOn ?? "—"}</td>
                  <td className="td">{o.items?.length ?? 0}</td>
                  <td className="td">
                    <Pill tone={o.status === "ISSUED" ? "green" : o.status === "REVOKED" ? "red" : "slate"}>
                      {o.status ?? "—"}
                    </Pill>
                  </td>
                </tr>
              ))}
            </Table>
          ) : (
            <EmptyState><T>No orders for this unit.</T></EmptyState>
          )}
          <Pager
            basePath="/orders"
            nextPageToken={page.nextPageToken}
            extraQuery={`unitId=${encodeURIComponent(unitId)}`}
          />
          <div className="mt-6 max-w-xl">
            <OrderCreate unitId={unitId} orderTypes={orderTypes} />
          </div>
        </>
      )}

      {!unitId && (
        <EmptyState><T>Pick an issuing unit above to list or create orders.</T></EmptyState>
      )}

      <div className="mt-8 max-w-xl">
        <OrderTypeManager types={orderTypes} />
      </div>
    </div>
  );
}
