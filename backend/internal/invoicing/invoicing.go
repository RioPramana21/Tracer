// Package invoicing generates an invoice for a placed order: the subtotal,
// the discount applied, the tax, and the resulting total the customer owes.
package invoicing

import (
	"context"
	"time"

	"alder.example/api/internal/pricing"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Invoice struct {
	ID            int       `json:"id"`
	OrderID       int       `json:"order_id"`
	SubtotalCents int64     `json:"subtotal_cents"`
	DiscountCents int64     `json:"discount_cents"`
	TaxCents      int64     `json:"tax_cents"`
	TotalCents    int64     `json:"total_cents"`
	CreatedAt     time.Time `json:"created_at"`
}

type Store struct {
	Pool *pgxpool.Pool
}

// Generate builds and stores the invoice for an order: it reads the order's
// lines and each product's unit price, then prices the order.
func (s *Store) Generate(ctx context.Context, orderID int) (Invoice, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT ol.product_id, ol.quantity, p.unit_price_cents
		 FROM order_lines ol JOIN products p ON p.id = ol.product_id
		 WHERE ol.order_id = $1`, orderID)
	if err != nil {
		return Invoice{}, err
	}
	unitPrices := map[int]int64{}
	quantities := map[int]int{}
	for rows.Next() {
		var productID, quantity int
		var unitPrice int64
		if err := rows.Scan(&productID, &quantity, &unitPrice); err != nil {
			rows.Close()
			return Invoice{}, err
		}
		unitPrices[productID] = unitPrice
		quantities[productID] += quantity
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Invoice{}, err
	}

	var discountBasisPoints int
	if err := s.Pool.QueryRow(ctx,
		`SELECT discount_basis_points FROM orders WHERE id = $1`, orderID,
	).Scan(&discountBasisPoints); err != nil {
		return Invoice{}, err
	}

	subtotal := pricing.Subtotal(unitPrices, quantities)
	discount := pricing.Discount(subtotal, discountBasisPoints)
	tax := pricing.Tax(subtotal - discount)
	total := subtotal - discount + tax

	var inv Invoice
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO invoices (order_id, subtotal_cents, discount_cents, tax_cents, total_cents)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, order_id, subtotal_cents, discount_cents, tax_cents, total_cents, created_at`,
		orderID, subtotal, discount, tax, total,
	).Scan(&inv.ID, &inv.OrderID, &inv.SubtotalCents, &inv.DiscountCents, &inv.TaxCents, &inv.TotalCents, &inv.CreatedAt)
	return inv, err
}

func (s *Store) GetByOrder(ctx context.Context, orderID int) (Invoice, error) {
	var inv Invoice
	err := s.Pool.QueryRow(ctx,
		`SELECT id, order_id, subtotal_cents, discount_cents, tax_cents, total_cents, created_at
		 FROM invoices WHERE order_id = $1`, orderID,
	).Scan(&inv.ID, &inv.OrderID, &inv.SubtotalCents, &inv.DiscountCents, &inv.TaxCents, &inv.TotalCents, &inv.CreatedAt)
	return inv, err
}
