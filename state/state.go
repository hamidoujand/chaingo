package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/genesis"
	"github.com/hamidoujand/chaingo/mempool"
	"github.com/hamidoujand/chaingo/peer"
)

var ErrNoTransaction = errors.New("no transaction inside mempool")

// QueryLatest represents to query the latest block in the chain.
const QueryLatest = ^uint64(0) >> 1

const (
	ConsensusPOA = "POA"
	ConsensusPOW = "POW"
)

type Config struct {
	//AccountID that receives the mining rewards for the Node.
	BeneficiaryID database.AccountID
	Genesis       genesis.Genesis
	Strategy      string
	Storage       database.Storage
	KnownPeers    *peer.PeerSet
	Host          string
	Consensus     string
}

// Worker represents the behavior required to do mining, peer update, shareTx across network.
type Worker interface {
	Shutdown()
	SignalStartMining()
	SignalCancelMining()
	SignalShareTX(tx database.BlockTX)
	Sync()
}

// State manages the blockchain database for us.
type State struct {
	mu            sync.RWMutex
	beneficiaryID database.AccountID
	genesis       genesis.Genesis
	db            *database.Database
	mempool       *mempool.Mempool
	Worker        Worker
	storage       database.Storage
	knownPeers    *peer.PeerSet
	host          string
	consensus     string
}

func New(conf Config) (*State, error) {
	db, err := database.New(conf.Genesis, conf.Storage)
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
		storage:       conf.Storage,
		knownPeers:    conf.KnownPeers,
		host:          conf.Host,
		consensus:     conf.Consensus,
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

func (s *State) Consensus() string {
	return s.consensus
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

	s.Worker.SignalShareTX(tx)
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
	if s.consensus == ConsensusPOA {
		difficulty = 1
	}

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

	if err := s.db.Write(block); err != nil {
		return fmt.Errorf("writing block to disk: %w", err)
	}

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

// KnownExternalPeers returns a copy of node hosts on the network excluding
// current node.
func (s *State) KnownExternalPeers() []peer.Peer {
	return s.knownPeers.Copy(s.host)
}

// KnownPeers retrieves a copy of the full known peer list which includes
// this node as well. Used by the PoA selection algorithm.
func (s *State) KnownPeers() []peer.Peer {
	return s.knownPeers.Copy("")
}

func (s *State) Host() string {
	return s.host
}

func (s *State) AddKnownPeer(peer peer.Peer) bool {
	return s.knownPeers.Add(peer)
}

func (s *State) RequestPeerStatus(p peer.Peer) (peer.PeerStatus, error) {
	log.Println("state: requestPeerStatus: started")
	defer log.Println("state: requestPeerStatus: completed")

	url := fmt.Sprintf("http://%s/node/status", p.Host)

	var peerStatus peer.PeerStatus
	if err := send(http.MethodGet, url, nil, &peerStatus); err != nil {
		return peer.PeerStatus{}, fmt.Errorf("send: %w", err)
	}
	log.Printf("state: RequestPeerStatus: HOST[%s]: latestBlockNumber[%d]: peersList: %v\n", p.Host, peerStatus.LatestBlockNumber, peerStatus.KnownPeers)
	return peerStatus, nil
}

func (s *State) RequestMempool(p peer.Peer) ([]database.BlockTX, error) {
	url := fmt.Sprintf("http://%s/node/tx/list", p.Host)

	var pool []database.BlockTX
	if err := send(http.MethodGet, url, nil, &pool); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	return pool, nil
}

func (s *State) RequestPeerBlocks(p peer.Peer) error {
	from := s.LatestBlock().Header.Number + 1 //from this block other nodes need to send blocks.

	url := fmt.Sprintf("http://%s/node/block/list/%d/latest", p.Host, from)

	var blocks []database.BlockData
	if err := send(http.MethodGet, url, nil, &blocks); err != nil {
		return fmt.Errorf("send: %w", err)
	}

	//convert into in memory block
	for _, blk := range blocks {
		block, err := database.ToBlock(blk)
		if err != nil {
			return fmt.Errorf("toBlock: %w", err)
		}

		//process the block
		if err := s.ProcessProposedBlock(block); err != nil {
			return fmt.Errorf("processProposedBlock: %w", err)
		}
	}

	return nil
}

func (s *State) QueryBlocksByNumber(from, to uint64) []database.Block {
	if from == QueryLatest {
		from = s.db.LatestBlock().Header.Number
		to = from
	}

	if to == QueryLatest {
		to = s.db.LatestBlock().Header.Number
	}

	var result []database.Block
	for i := from; i <= to; i++ {
		block, err := s.db.GetBlock(i)
		if err != nil {
			log.Printf("getting block: %s", err)
			return nil
		}

		result = append(result, block)
	}

	return result
}

func (s *State) SendNodeAvailableToPeers() {
	p := peer.Peer{Host: s.Host()}

	for _, peer := range s.KnownExternalPeers() {
		url := fmt.Sprintf("http://%s/node/peers", peer.Host)
		if err := send(http.MethodPost, url, p, nil); err != nil {
			log.Printf("sendNodeAvailableToPeers: Error: %v", err)
		}
	}
}

func (s *State) ProcessProposedBlock(block database.Block) error {
	//also when other nodes mined a new block and proposed it to this node
	// we first validate and then cancel our POW
	if err := s.validateAndUpdateDB(block); err != nil {
		return fmt.Errorf("validateAndUpdateDB: %w", err)
	}

	s.Worker.SignalCancelMining()
	return nil
}

func (s *State) SendTxToPeers(tx database.BlockTX) {

	for _, peer := range s.KnownExternalPeers() {
		url := fmt.Sprintf("http://%s/node/tx/submit", peer.Host)
		if err := send(http.MethodPost, url, tx, nil); err != nil {
			log.Printf("sendTxToPeers: ERROR: %v", err)
		}
	}
}

func (s *State) UpsertNodeTransaction(tx database.BlockTX) error {
	//validate
	if err := tx.Validate(s.genesis.ChainID); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := s.UpsertMempool(tx); err != nil {
		return fmt.Errorf("upsertMempool: %w", err)
	}

	//signal mining
	s.Worker.SignalStartMining()

	return nil
}

func (s *State) SendBlockToPeers(block database.Block) error {
	for _, peer := range s.KnownExternalPeers() {
		url := fmt.Sprintf("http://%s/node/block/propose", peer.Host)

		var result struct {
			Status string `json:"status"`
		}
		if err := send(http.MethodPost, url, database.NewBlockData(block), &result); err != nil {
			return fmt.Errorf("send: peer[%s]: %w", peer.Host, err)
		}
	}

	return nil
}

// RemoveKnownPeer provides the ability to remove a peer from
// the known peer list.
func (s *State) RemoveKnownPeer(peer peer.Peer) {
	s.knownPeers.Remove(peer)
}

// ==============================================================================
func send(method string, url string, dataSend any, dataReceive any) error {
	var req *http.Request

	switch {
	case dataSend != nil:
		data, err := json.Marshal(dataSend)
		if err != nil {
			return fmt.Errorf("marshalling: %w", err)
		}

		req, err = http.NewRequest(method, url, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}
	default:
		var err error
		req, err = http.NewRequest(method, url, nil)
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}
	}

	//TODO: configure a custom client
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		//have an err
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("readAll: %w", err)
		}

		var errResponse struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(bs, &errResponse); err != nil {
			return fmt.Errorf("unmarshalErr: %w", err)
		}

		return errors.New(errResponse.Error)
	}

	if dataReceive != nil {
		if err := json.NewDecoder(resp.Body).Decode(dataReceive); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
	}

	return nil
}
