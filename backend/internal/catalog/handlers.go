package catalog

import (
	"encoding/json"
	"errors"
	"net/http"
)

type createRequest struct {
	SKU            string `json:"sku"`
	Name           string `json:"name"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

func (s *Store) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	p, err := s.Create(r.Context(), req.SKU, req.Name, req.UnitPriceCents)
	if errors.Is(err, ErrDuplicateSKU) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Store) HandleList(w http.ResponseWriter, r *http.Request) {
	products, err := s.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, products)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
