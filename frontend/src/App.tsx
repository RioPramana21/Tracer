import { useState } from "react"
import { OrderDetail } from "./OrderDetail"
import { OrderList } from "./OrderList"

function App() {
  const [selectedId, setSelectedId] = useState<number | null>(null)

  return (
    <div className="app">
      <h1>Alder</h1>
      {selectedId === null ? (
        <OrderList onSelect={setSelectedId} />
      ) : (
        <div>
          <button onClick={() => setSelectedId(null)}>&larr; Back to orders</button>
          <OrderDetail orderId={selectedId} />
        </div>
      )}
    </div>
  )
}

export default App
