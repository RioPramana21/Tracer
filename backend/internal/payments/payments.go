// Package payments records payments taken against an invoice.
package payments

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Payment struct {
	ID          int       `json:"id"`
	InvoiceID   int       `json:"invoice_id"`
	AmountCents int64     `json:"amount_cents"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store struct {
	Pool *pgxpool.Pool
}

// Record takes a payment for the full total owed on an invoice.
func (s *Store) Record(ctx context.Context, invoiceID int) (Payment, error) {
	var totalCents int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT total_cents FROM invoices WHERE id = $1`, invoiceID,
	).Scan(&totalCents); err != nil {
		return Payment{}, err
	}

	var p Payment
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO payments (invoice_id, amount_cents, status) VALUES ($1, $2, 'paid')
		 RETURNING id, invoice_id, amount_cents, status, created_at`,
		invoiceID, totalCents,
	).Scan(&p.ID, &p.InvoiceID, &p.AmountCents, &p.Status, &p.CreatedAt)
	return p, err
}

func (s *Store) GetByInvoice(ctx context.Context, invoiceID int) (Payment, error) {
	var p Payment
	err := s.Pool.QueryRow(ctx,
		`SELECT id, invoice_id, amount_cents, status, created_at FROM payments WHERE invoice_id = $1`,
		invoiceID,
	).Scan(&p.ID, &p.InvoiceID, &p.AmountCents, &p.Status, &p.CreatedAt)
	return p, err
}
