package handlers

import (
	"net/http"

	"github.com/hamidoujand/chaingo/handlers/private"
	"github.com/hamidoujand/chaingo/handlers/public"
	"github.com/hamidoujand/chaingo/nameservice"
	"github.com/hamidoujand/chaingo/state"
)

type MuxConfig struct {
	State *state.State
	NS    *nameservice.Nameservice
}

func PublicMux(conf MuxConfig) http.Handler {
	mux := http.NewServeMux()

	h := public.Handlers{
		State: conf.State,
		NS:    conf.NS,
	}

	mux.HandleFunc("GET /genesis/list", h.Genesis)
	mux.HandleFunc("GET /accounts/list", h.Accounts)
	mux.HandleFunc("GET /accounts/list/{accountID}", h.Accounts)
	mux.HandleFunc("GET /transactions/uncommit/list", h.Mempool)
	mux.HandleFunc("GET /transactions/uncommit/list/{accountID}", h.Mempool)
	mux.HandleFunc("POST /transactions/submit", h.SubmitTX)

	return mux
}

func PrivateMux(conf MuxConfig) http.Handler {
	mux := http.NewServeMux()

	prv := private.Handlers{
		State: conf.State,
		NS:    conf.NS,
	}

	mux.HandleFunc("GET /node/status", prv.Status)
	mux.HandleFunc("GET /node/tx/list", prv.Mempool)
	mux.HandleFunc("GET /node/block/list/{from}/{to}", prv.BlocksByNumber)
	mux.HandleFunc("POST /node/peers", prv.SubmitPeer)

	return mux
}
