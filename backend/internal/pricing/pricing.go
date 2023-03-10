// Package pricing computes the money amounts on an order: subtotal, tax, and
// (later) discount. All amounts are in cents to avoid floating point drift.
package pricing

const taxRateBasisPoints = 1100 // 11%

// Subtotal sums quantity * unit price across the given lines.
func Subtotal(unitPricesCents map[int]int64, quantities map[int]int) int64 {
	var total int64
	for productID, qty := range quantities {
		total += unitPricesCents[productID] * int64(qty)
	}
	return total
}

// Tax applies the flat tax rate to a subtotal.
func Tax(subtotalCents int64) int64 {
	return subtotalCents * taxRateBasisPoints / 10000
}
