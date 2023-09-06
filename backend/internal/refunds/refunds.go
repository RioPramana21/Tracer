// Package refunds issues a refund against a payment.
package refunds

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Refund struct {
	ID          int       `json:"id"`
	PaymentID   int       `json:"payment_id"`
	AmountCents int64     `json:"amount_cents"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store struct {
	Pool *pgxpool.Pool
}

// Issue refunds a payment in full and records why.
func (s *Store) Issue(ctx context.Context, paymentID int, reason string) (Refund, error) {
	var invoiceID int
	if err := s.Pool.QueryRow(ctx,
		`SELECT invoice_id FROM payments WHERE id = $1`, paymentID,
	).Scan(&invoiceID); err != nil {
		return Refund{}, err
	}

	var amountCents int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT subtotal_cents FROM invoices WHERE id = $1`, invoiceID,
	).Scan(&amountCents); err != nil {
		return Refund{}, err
	}

	var r Refund
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO refunds (payment_id, amount_cents, reason) VALUES ($1, $2, $3)
		 RETURNING id, payment_id, amount_cents, reason, created_at`,
		paymentID, amountCents, reason,
	).Scan(&r.ID, &r.PaymentID, &r.AmountCents, &r.Reason, &r.CreatedAt)
	return r, err
}

func (s *Store) GetByPayment(ctx context.Context, paymentID int) (Refund, error) {
	var r Refund
	err := s.Pool.QueryRow(ctx,
		`SELECT id, payment_id, amount_cents, reason, created_at FROM refunds WHERE payment_id = $1`,
		paymentID,
	).Scan(&r.ID, &r.PaymentID, &r.AmountCents, &r.Reason, &r.CreatedAt)
	return r, err
}
