// Package public maintains the group of handlers for public access.
package public

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hamidoujand/chaingo/state"
)

type Handlers struct {
	State *state.State
}

func (h *Handlers) Genesis(w http.ResponseWriter, r *http.Request) {
	gen := h.State.Genesis()
	if err := respond(w, http.StatusOK, gen); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (h *Handlers) Accounts(w http.ResponseWriter, r *http.Request) {

}

func (h *Handlers) Mempool(w http.ResponseWriter, r *http.Request) {

}

func (h *Handlers) SubmitTX(w http.ResponseWriter, r *http.Request) {

}

func respond(w http.ResponseWriter, statusCode int, data any) error {
	w.WriteHeader(statusCode)
	if statusCode == http.StatusNoContent {
		return nil
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(data); err != nil {
		return fmt.Errorf("encoding data into json: %w", err)
	}

	return nil
}
