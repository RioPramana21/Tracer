import { useEffect, useState } from "react"
import { getInvoice, getOrder, type Invoice, type Order } from "./api"

function cents(v: number) {
  return `$${(v / 100).toFixed(2)}`
}

export function OrderDetail({ orderId }: { orderId: number }) {
  const [order, setOrder] = useState<Order | null>(null)
  const [invoice, setInvoice] = useState<Invoice | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getOrder(orderId).then(setOrder).catch((e) => setError(e.message))
    getInvoice(orderId)
      .then(setInvoice)
      .catch(() => setInvoice(null))
  }, [orderId])

  if (error) return <p>Couldn't load order: {error}</p>
  if (!order) return <p>Loading…</p>

  return (
    <div>
      <h2>
        Order #{order.id} — {order.customer_name}
      </h2>
      <p>Status: {order.status}</p>
      {invoice && (
        <ul>
          <li>Subtotal: {cents(invoice.subtotal_cents)}</li>
          <li>Discount: {cents(invoice.discount_cents)}</li>
          <li>Tax: {cents(invoice.tax_cents)}</li>
          <li>
            <strong>Total: {cents(invoice.total_cents)}</strong>
          </li>
        </ul>
      )}
    </div>
  )
}
