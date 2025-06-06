package public

import "github.com/hamidoujand/chaingo/database"

type tx struct {
	FromAccount database.AccountID `json:"from"`
	FromName    string             `json:"from_name"`
	ToAccount   database.AccountID `json:"to"`
	ToName      string             `json:"to_name"`
	ChainID     uint16             `json:"chain_id"`
	Nonce       uint64             `json:"nonce"`
	Value       uint64             `json:"value"`
	Tip         uint64             `json:"tip"`
	Data        []byte             `json:"data"`
	TimeStamp   uint64             `json:"timestamp"`
	GasPrice    uint64             `json:"gas_price"`
	GasUnits    uint64             `json:"gas_units"`
	Sig         string             `json:"sig"`
}

type account struct {
	Account database.AccountID `json:"account"`
	Name    string             `json:"name"`
	Balance uint64             `json:"balance"`
	Nonce   uint64             `json:"nonce"`
}

type accountInfo struct {
	LatestBlock string    `json:"latest_block"`
	Uncommitted int       `json:"uncommitted"`
	Accounts    []account `json:"accounts"`
}
