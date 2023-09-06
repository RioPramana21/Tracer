package refunds

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type issueRequest struct {
	Reason string `json:"reason"`
}

func (s *Store) HandleIssue(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid payment id", http.StatusBadRequest)
		return
	}
	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	refund, err := s.Issue(r.Context(), paymentID, req.Reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, refund)
}

func (s *Store) HandleGet(w http.ResponseWriter, r *http.Request) {
	paymentID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid payment id", http.StatusBadRequest)
		return
	}
	refund, err := s.GetByPayment(r.Context(), paymentID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, refund)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
