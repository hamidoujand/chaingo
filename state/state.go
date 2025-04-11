package state

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/genesis"
	"github.com/hamidoujand/chaingo/mempool"
)

var ErrNoTransaction = errors.New("no transaction inside mempool")

type Config struct {
	//AccountID that receives the mining rewards for the Node.
	BeneficiaryID database.AccountID
	Genesis       genesis.Genesis
	Strategy      string
}

// Worker represents the behavior required to do mining, peer update, shareTx across network.
type Worker interface {
	Shutdown()
	SignalStartMining()
	SignalCancelMining()
}

// State manages the blockchain database for us.
type State struct {
	mu            sync.RWMutex
	beneficiaryID database.AccountID
	genesis       genesis.Genesis
	db            *database.Database
	mempool       *mempool.Mempool
	Worker        Worker
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

func (s *State) QueryAccount(accountID database.AccountID) (database.Account, error) {
	return s.db.Query(accountID)
}

func (s *State) Genesis() genesis.Genesis {
	return s.genesis
}

func (s *State) UpsertWalletTransaction(signedTX database.SignedTX) error {
	//It's up to the wallet to make sure the account has a proper
	// balance and this transaction has a proper nonce. Fees will be taken if
	// this transaction is mined into a block it doesn't have enough money to
	// pay or the nonce isn't the next expected nonce for the account.

	//validate the signature
	if err := signedTX.Validate(s.genesis.ChainID); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	//create a blockTx from it
	const oneUnitOfGas = 1
	tx := database.NewBlockTX(signedTX, s.genesis.GasPrice, oneUnitOfGas)

	//insert into mempool.
	if err := s.mempool.Upsert(tx); err != nil {
		return fmt.Errorf("upsert blockTX into mempool: %w", err)
	}

	//TODO: share TX with rest of the network
	s.Worker.SignalStartMining()

	return nil
}

func (s *State) MineNewBlock(ctx context.Context) (database.Block, error) {
	defer log.Println("mined a new block")

	if s.mempool.Count() == 0 {
		return database.Block{}, ErrNoTransaction
	}

	//peek best transactions
	trans := s.mempool.PickBest(int(s.genesis.TransPerBlock))

	difficulty := s.genesis.Difficulty

	block, err := database.POW(ctx, database.POWConf{
		BeneficiaryID: s.beneficiaryID,
		Difficulty:    difficulty,
		MiningReward:  s.genesis.MiningReward,
		PrevBlock:     s.db.LatestBlock(),
		StateRoot:     s.db.HashState(),
		Trans:         trans,
	})

	if err != nil {
		return database.Block{}, fmt.Errorf("pow: %w", err)
	}

	//check to see if we are not cancelled
	if err := ctx.Err(); err != nil {
		return database.Block{}, err
	}

	//validate the block and then write to the db
	if err := s.validateAndUpdateDB(block); err != nil {
		return database.Block{}, fmt.Errorf("validateAndUpdateDB: %w", err)
	}

	return block, nil
}

func (s *State) Shutdown() error {
	//stop all blockchain writing activity
	s.Worker.Shutdown()

	return nil
}

// validateAndUpdateDB validates the block against consensus rules. if block
// passes validation block will be added to disk.
// this function will be used for both validation of blocks that our node mined
// or the blocks the node gets from other peers.
func (s *State) validateAndUpdateDB(block database.Block) error {
	//need this lock in here, in case our node mined a block and needs to validate
	//and at the same time we have a block coming on private peer to peer network
	// we only handle one at the time.
	//since http request is on its own G, and our worker also creates a G to mine
	// a new block.
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Println("state: validateAndUpdateDB: validating block")

	if err := block.ValidateBlock(s.db.LatestBlock(), s.db.HashState()); err != nil {
		return fmt.Errorf("validationBlock: %w", err)
	}

	//TODO: write to the disk

	//updated latest block
	s.db.UpdateLatestBlock(block)

	//process transaction and apply accounting
	for _, tx := range block.MerkleTree.Values() {
		log.Printf("applying and removing tx: %s\n", tx)

		//remove from mempool
		s.mempool.Delete(tx)

		if err := s.db.ApplyTransaction(block, tx); err != nil {
			log.Printf("failed to apply transaction %s: %s\n", tx, err)
			continue
		}
	}

	//apply mining reward to the account that mined the block
	s.db.ApplyMiningReward(block)

	return nil
}

// LatestBlock returns the latest block.
func (s *State) LatestBlock() database.Block {
	return s.db.LatestBlock()
}
