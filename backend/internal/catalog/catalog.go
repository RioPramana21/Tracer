// Package catalog manages the product catalog.
package catalog

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Product struct {
	ID             int       `json:"id"`
	SKU            string    `json:"sku"`
	Name           string    `json:"name"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	CreatedAt      time.Time `json:"created_at"`
}

type Store struct {
	Pool *pgxpool.Pool
}

func (s *Store) Create(ctx context.Context, sku, name string, unitPriceCents int64) (Product, error) {
	var p Product
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO products (sku, name, unit_price_cents) VALUES ($1, $2, $3)
		 RETURNING id, sku, name, unit_price_cents, created_at`,
		sku, name, unitPriceCents,
	).Scan(&p.ID, &p.SKU, &p.Name, &p.UnitPriceCents, &p.CreatedAt)
	return p, err
}

func (s *Store) List(ctx context.Context) ([]Product, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, sku, name, unit_price_cents, created_at FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.UnitPriceCents, &p.CreatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (s *Store) Get(ctx context.Context, id int) (Product, error) {
	var p Product
	err := s.Pool.QueryRow(ctx,
		`SELECT id, sku, name, unit_price_cents, created_at FROM products WHERE id = $1`, id,
	).Scan(&p.ID, &p.SKU, &p.Name, &p.UnitPriceCents, &p.CreatedAt)
	return p, err
}
