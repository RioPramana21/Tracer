package main

import (
	"context"
	"log"
	"net/http"

	"alder.example/api/internal/catalog"
	"alder.example/api/internal/db"
	"alder.example/api/internal/invoicing"
	"alder.example/api/internal/orders"
	"alder.example/api/internal/payments"
	"alder.example/api/internal/refunds"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func main() {
	ctx := context.Background()

	pool, err := db.Open(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	products := &catalog.Store{Pool: pool}
	orderStore := &orders.Store{Pool: pool}
	invoiceStore := &invoicing.Store{Pool: pool}
	paymentStore := &payments.Store{Pool: pool}
	refundStore := &refunds.Store{Pool: pool}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}))
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	r.Post("/products", products.HandleCreate)
	r.Get("/products", products.HandleList)
	r.Post("/orders", orderStore.HandlePlace)
	r.Get("/orders", orderStore.HandleList)
	r.Get("/orders/{id}", orderStore.HandleGet)
	r.Post("/orders/{id}/invoice", invoiceStore.HandleGenerate)
	r.Get("/orders/{id}/invoice", invoiceStore.HandleGet)
	r.Post("/invoices/{id}/payment", paymentStore.HandleRecord)
	r.Get("/invoices/{id}/payment", paymentStore.HandleGet)
	r.Post("/payments/{id}/refund", refundStore.HandleIssue)
	r.Get("/payments/{id}/refund", refundStore.HandleGet)

	log.Println("alder api listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
