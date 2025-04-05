// Package public maintains the group of handlers for public access.
package public

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/state"
)

type Handlers struct {
	State *state.State
}

func (h *Handlers) Genesis(w http.ResponseWriter, r *http.Request) {
	gen := h.State.Genesis()
	if err := respond(w, http.StatusOK, gen); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		respond(w, http.StatusInternalServerError, ErrorResponse{Error: http.StatusText(http.StatusInternalServerError)})
	}
}

func (h *Handlers) Accounts(w http.ResponseWriter, r *http.Request) {
	accountSTR := r.PathValue("accountID")

	var accounts map[database.AccountID]database.Account

	switch accountSTR {
	case "":
		//all accounts
		accounts = h.State.Accounts()
	default:
		//specific account
		accountID, err := database.NewAccountID(accountSTR)
		if err != nil {
			msg := fmt.Sprintf("invalid accountID: %s", accountSTR)
			respond(w, http.StatusBadRequest, ErrorResponse{msg})
			return
		}

		account, err := h.State.QueryAccount(accountID)
		if err != nil {
			respond(w, http.StatusNotFound, ErrorResponse{fmt.Sprintf("account %s not found", accountSTR)})
			return
		}

		accounts = map[database.AccountID]database.Account{accountID: account}
	}

	respond(w, http.StatusOK, accounts)
}

func (h *Handlers) Mempool(w http.ResponseWriter, r *http.Request) {
	accountSTR := r.PathValue("accountID")

	mempool := h.State.Mempool()

	transactions := make([]tx, 0, len(mempool))

	for _, blockTX := range mempool {

		//if accountID is present then only include that tx if accountID is either equal to FromID or ToID.
		if (accountSTR != "") && ((database.AccountID(accountSTR) != blockTX.FromID) && database.AccountID(accountSTR) != blockTX.ToID) {
			continue
		}

		t := tx{
			FromAccount: blockTX.FromID,
			To:          blockTX.ToID,
			ChainID:     blockTX.ChainID,
			Nonce:       blockTX.Nonce,
			Value:       blockTX.Value,
			Tip:         blockTX.Tip,
			Data:        blockTX.Data,
			TimeStamp:   blockTX.Timestamp,
			GasPrice:    blockTX.GasPrice,
			GasUnits:    blockTX.GasUnits,
			Sig:         blockTX.SignatureString(),
		}

		transactions = append(transactions, t)
	}

	respond(w, http.StatusOK, transactions)
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

type ErrorResponse struct {
	Error string `json:"error"`
}
