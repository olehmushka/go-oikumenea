import Link from "next/link";
import { apiGet } from "@/lib/api/server";
import { EmptyState, ErrorNotice, Mono, Pager, PageHeader, Pill, Table } from "@/components/ui";
import { LookupForm } from "@/components/LookupForm";
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
  const orderTypes = await apiGet<OrderType[]>("/order/v1/order-types").catch(() => []);
  if (unitId) {
    try {
      const qs = new URLSearchParams({ pageSize: "50" });
      if (pageToken) qs.set("pageToken", pageToken);
      page = await apiGet<OrderPage>(`/order/v1/units/${unitId}/orders`, `?${qs}`);
    } catch (e) {
      error = e;
    }
  }

  return (
    <div>
      <PageHeader
        title="Orders"
        description="Administrative orders (наказ) — the legal basis for status changes. Effects apply on issue, with provenance."
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
                  <th className="th">Number</th>
                  <th className="th">Issued on</th>
                  <th className="th">Items</th>
                  <th className="th">Status</th>
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
            <EmptyState>No orders for this unit.</EmptyState>
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
        <EmptyState>Pick an issuing unit above to list or create orders.</EmptyState>
      )}

      <div className="mt-8 max-w-xl">
        <OrderTypeManager types={orderTypes} />
      </div>
    </div>
  );
}
