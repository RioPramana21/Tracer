// Package orders manages orders, their lines, and stock reservation.
package orders

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Line struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

type Order struct {
	ID           int       `json:"id"`
	CustomerName string    `json:"customer_name"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	Lines        []Line    `json:"lines,omitempty"`
}

type Store struct {
	Pool *pgxpool.Pool
}

// Place creates an order, its lines, and a stock reservation for each line,
// all inside one transaction.
func (s *Store) Place(ctx context.Context, customerName string, lines []Line) (Order, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	var o Order
	o.CustomerName = customerName
	o.Status = "placed"
	err = tx.QueryRow(ctx,
		`INSERT INTO orders (customer_name, status) VALUES ($1, $2)
		 RETURNING id, customer_name, status, created_at`,
		customerName, o.Status,
	).Scan(&o.ID, &o.CustomerName, &o.Status, &o.CreatedAt)
	if err != nil {
		return Order{}, err
	}

	for _, line := range lines {
		if _, err := tx.Exec(ctx,
			`INSERT INTO order_lines (order_id, product_id, quantity) VALUES ($1, $2, $3)`,
			o.ID, line.ProductID, line.Quantity,
		); err != nil {
			return Order{}, err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO stock_reservations (order_id, product_id, quantity) VALUES ($1, $2, $3)`,
			o.ID, line.ProductID, line.Quantity,
		); err != nil {
			return Order{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	o.Lines = lines
	return o, nil
}

func (s *Store) List(ctx context.Context) ([]Order, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, customer_name, status, created_at FROM orders ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.CustomerName, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, id int) (Order, error) {
	var o Order
	err := s.Pool.QueryRow(ctx,
		`SELECT id, customer_name, status, created_at FROM orders WHERE id = $1`, id,
	).Scan(&o.ID, &o.CustomerName, &o.Status, &o.CreatedAt)
	if err != nil {
		return Order{}, err
	}

	rows, err := s.Pool.Query(ctx,
		`SELECT product_id, quantity FROM order_lines WHERE order_id = $1`, id)
	if err != nil {
		return Order{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Line
		if err := rows.Scan(&l.ProductID, &l.Quantity); err != nil {
			return Order{}, err
		}
		o.Lines = append(o.Lines, l)
	}
	return o, rows.Err()
}

var ErrNotFound = pgx.ErrNoRows
