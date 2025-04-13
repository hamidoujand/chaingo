// Package private provides all handlers required for nodes communication inside the network.
package private

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/nameservice"
	"github.com/hamidoujand/chaingo/peer"
	"github.com/hamidoujand/chaingo/state"
)

type Handlers struct {
	State *state.State
	NS    *nameservice.Nameservice
}

func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	latest := h.State.LatestBlock()

	status := peer.PeerStatus{
		LatestBlockHash:   latest.Hash(),
		LatestBlockNumber: latest.Header.Number,
		KnownPeers:        h.State.KnownExternalPeers(),
	}

	if err := respond(w, http.StatusOK, status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handlers) Mempool(w http.ResponseWriter, r *http.Request) {
	tx := h.State.Mempool()

	if err := respond(w, http.StatusOK, tx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handlers) BlocksByNumber(w http.ResponseWriter, r *http.Request) {
	fromStr := r.PathValue("from")
	toStr := r.PathValue("to")

	if fromStr == "latest" || fromStr == "" {
		fromStr = fmt.Sprintf("%d", state.QueryLatest)
	}

	if toStr == "latest" || toStr == "" {
		toStr = fmt.Sprintf("%d", state.QueryLatest)
	}

	from, err := strconv.ParseUint(fromStr, 10, 64)
	if err != nil {
		respond(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	to, err := strconv.ParseUint(toStr, 10, 64)
	if err != nil {
		respond(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if from > to {
		respond(w, http.StatusBadRequest, ErrorResponse{Error: "from can not be greater than to"})
		return
	}

	blocks := h.State.QueryBlocksByNumber(from, to)
	if len(blocks) == 0 {
		respond(w, http.StatusNoContent, nil)
		return
	}

	blockData := make([]database.BlockData, len(blocks))
	for i, block := range blocks {
		blockData[i] = database.NewBlockData(block)
	}

	respond(w, http.StatusOK, blockData)
}

func (h *Handlers) SubmitPeer(w http.ResponseWriter, r *http.Request) {
	var peer peer.Peer
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		respond(w, http.StatusBadRequest, ErrorResponse{"failed to decode request body"})
		return
	}

	if !h.State.AddKnownPeer(peer) {
		log.Printf("adding new peer: %s\n", peer.Host)
	}

	respond(w, http.StatusOK, nil)
}

func (h *Handlers) SubmitTransaction(w http.ResponseWriter, r *http.Request) {
	var tx database.BlockTX
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		respond(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.State.UpsertNodeTransaction(tx); err != nil {
		respond(w, http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	msg := struct {
		Status string `json:"status"`
	}{
		Status: "transaction added to mempool",
	}

	respond(w, http.StatusOK, msg)
}

// =============================================================================
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

type ErrorResponse struct {
	Error string `json:"error"`
}
