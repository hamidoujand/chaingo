package state

import (
	"fmt"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/genesis"
)

type Config struct {
	//AccountID that receives the mining rewards for the Node.
	BeneficiaryID database.AccountID
	Genesis       genesis.Genesis
}

// State manages the blockchain database for us.
type State struct {
	beneficiaryID database.AccountID
	genesis       genesis.Genesis
	db            *database.Database
}

func New(conf Config) (*State, error) {
	db, err := database.New(conf.Genesis)
	if err != nil {
		return nil, fmt.Errorf("new database: %w", err)
	}

	s := State{
		beneficiaryID: conf.BeneficiaryID,
		genesis:       conf.Genesis,
		db:            db,
	}

	return &s, nil
}
