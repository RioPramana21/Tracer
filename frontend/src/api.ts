const BASE_URL = "http://localhost:8080"

export type OrderLine = {
  product_id: number
  quantity: number
}

export type Order = {
  id: number
  customer_name: string
  status: string
  discount_basis_points: number
  created_at: string
  lines?: OrderLine[]
}

export type Invoice = {
  id: number
  order_id: number
  subtotal_cents: number
  discount_cents: number
  tax_cents: number
  total_cents: number
}

export async function listOrders(): Promise<Order[]> {
  const res = await fetch(`${BASE_URL}/orders`)
  if (!res.ok) throw new Error("failed to load orders")
  return res.json()
}

export async function getOrder(id: number): Promise<Order> {
  const res = await fetch(`${BASE_URL}/orders/${id}`)
  if (!res.ok) throw new Error("failed to load order")
  return res.json()
}

export async function getInvoice(orderId: number): Promise<Invoice> {
  const res = await fetch(`${BASE_URL}/orders/${orderId}/invoice`)
  if (!res.ok) throw new Error("failed to load invoice")
  return res.json()
}

export type Payment = {
  id: number
  invoice_id: number
  amount_cents: number
  status: string
}

export async function getPayment(invoiceId: number): Promise<Payment> {
  const res = await fetch(`${BASE_URL}/invoices/${invoiceId}/payment`)
  if (!res.ok) throw new Error("failed to load payment")
  return res.json()
}
