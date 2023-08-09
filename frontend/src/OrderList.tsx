import { useEffect, useState } from "react"
import { listOrders, type Order } from "./api"

const PAGE_SIZE = 20

export function OrderList({ onSelect }: { onSelect: (id: number) => void }) {
  const [orders, setOrders] = useState<Order[]>([])
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(0)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    listOrders(PAGE_SIZE, page * PAGE_SIZE)
      .then(setOrders)
      .catch((e) => setError(e.message))
  }, [page])

  if (error) return <p>Couldn't load orders: {error}</p>

  const visible = orders.filter((o) =>
    o.customer_name.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div>
      <input
        placeholder="Search by customer"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <table>
        <thead>
          <tr>
            <th>Order</th>
            <th>Customer</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {visible.map((o) => (
            <tr key={o.id} onClick={() => onSelect(o.id)} style={{ cursor: "pointer" }}>
              <td>#{o.id}</td>
              <td>{o.customer_name}</td>
              <td>{o.status}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <button disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
        Previous
      </button>
      <button disabled={orders.length < PAGE_SIZE} onClick={() => setPage((p) => p + 1)}>
        Next
      </button>
    </div>
  )
}
