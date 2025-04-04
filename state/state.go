package state

import (
	"fmt"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/genesis"
	"github.com/hamidoujand/chaingo/mempool"
)

type Config struct {
	//AccountID that receives the mining rewards for the Node.
	BeneficiaryID database.AccountID
	Genesis       genesis.Genesis
	Strategy      string
}

// State manages the blockchain database for us.
type State struct {
	beneficiaryID database.AccountID
	genesis       genesis.Genesis
	db            *database.Database
	mempool       *mempool.Mempool
}

func New(conf Config) (*State, error) {
	db, err := database.New(conf.Genesis)
	if err != nil {
		return nil, fmt.Errorf("new database: %w", err)
	}

	mempool, err := mempool.New(conf.Strategy)
	if err != nil {
		return nil, fmt.Errorf("new mempool: %w", err)
	}

	s := State{
		beneficiaryID: conf.BeneficiaryID,
		genesis:       conf.Genesis,
		db:            db,
		mempool:       mempool,
	}

	return &s, nil
}

func (s *State) MempoolLen() int {
	return s.mempool.Count()
}

func (s *State) Mempool() []database.BlockTX {
	return s.mempool.PickBest(0) //count=0, means all transactions .
}

func (s *State) UpsertMempool(tx database.BlockTX) error {
	return s.mempool.Upsert(tx)
}

func (s *State) Accounts() map[database.AccountID]database.Account {
	return s.db.Copy()
}
