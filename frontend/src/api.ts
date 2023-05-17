const BASE_URL = "http://localhost:8080"

export type Order = {
  id: number
  customer_name: string
  status: string
  discount_basis_points: number
  created_at: string
}

export async function listOrders(): Promise<Order[]> {
  const res = await fetch(`${BASE_URL}/orders`)
  if (!res.ok) throw new Error("failed to load orders")
  return res.json()
}
