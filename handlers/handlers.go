package handlers

import (
	"net/http"

	"github.com/hamidoujand/chaingo/handlers/public"
	"github.com/hamidoujand/chaingo/state"
)

type MuxConfig struct {
	State *state.State
}

func PublicMux(conf MuxConfig) http.Handler {
	mux := http.NewServeMux()

	h := public.Handlers{
		State: conf.State,
	}

	mux.HandleFunc("GET /genesis/list", h.Genesis)
	mux.HandleFunc("GET /accounts/list", h.Accounts)
	mux.HandleFunc("GET /accounts/list/{accountID}", h.Accounts)
	mux.HandleFunc("GET /transactions/uncommit/list", h.Mempool)
	mux.HandleFunc("POST /transactions/submit", h.SubmitTX)

	return mux
}
