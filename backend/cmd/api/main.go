package main

import (
	"context"
	"log"
	"net/http"

	"alder.example/api/internal/catalog"
	"alder.example/api/internal/db"
	"github.com/go-chi/chi/v5"
)

func main() {
	ctx := context.Background()

	pool, err := db.Open(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	products := &catalog.Store{Pool: pool}

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	r.Post("/products", products.HandleCreate)
	r.Get("/products", products.HandleList)

	log.Println("alder api listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
