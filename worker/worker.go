// Package worker provides concurrent support for mining, peer updates, tx sharing.
package worker

import (
	"context"
	"errors"
	"hash/fnv"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/peer"
	"github.com/hamidoujand/chaingo/state"
)

// maxTXSharing is max number of tx that can be batched and more than this will be dropped.
const maxTXSharingRequest = 100 //TODO: requires load testing to find the correct buffer size.

// peerUpdateInterval represents the interval of finding new peer nodes
// and updating the blockchain on disk with missing blocks.
const peerUpdateInterval = time.Second * 10

// cycleDuration sets the mining operation to happen every 12 seconds in poa
const secondsPerCycle = 12
const cycleDuration = secondsPerCycle * time.Second

// Worker is the manager of all goroutines created by this package.
type Worker struct {
	state        *state.State
	wg           sync.WaitGroup
	shutdown     chan struct{}
	startMining  chan bool
	cancelMining chan bool
	txSharing    chan database.BlockTX
	ticker       *time.Ticker
}

// Run creates a worker and attach the worker to the state.
func Run(st *state.State) {
	w := Worker{
		state:        st,
		shutdown:     make(chan struct{}),
		startMining:  make(chan bool, 1),
		cancelMining: make(chan bool, 1),
		txSharing:    make(chan database.BlockTX, maxTXSharingRequest),
		ticker:       time.NewTicker(peerUpdateInterval),
	}

	//register worker to the state
	st.Worker = &w

	//sync the node with the rest of network
	w.Sync()

	consensusOperation := w.PowOperation

	if st.Consensus() == state.ConsensusPOA {
		consensusOperation = w.poaOperation
	}

	//set of operation to do
	operations := []func(){
		w.peerSyncOperation,
		w.ShareTxOperation,
		consensusOperation,
	}

	g := len(operations)
	w.wg.Add(g)

	//has started
	hasStarted := make(chan bool)

	for _, op := range operations {
		go func() {
			defer w.wg.Done()
			hasStarted <- true
			op()
		}()
	}

	//wait till all operations are at least started
	for range g {
		<-hasStarted
	}
}

// Shutdown implements state.Worker.
func (w *Worker) Shutdown() {
	log.Println("worker shutdown started")
	defer log.Println("worker shutdown completed")

	w.SignalCancelMining()

	log.Println("terminating worker goroutines")
	close(w.shutdown)
	w.wg.Wait()
}

// SignalCancelMining implements state.Worker.
func (w *Worker) SignalCancelMining() {
	select {
	case w.cancelMining <- true:
	default:

	}
	log.Println("worker: signaling the cancel mining")
}

// SignalStartMining implements state.Worker is there is a signal already pending
// some other call signaled before , no need to signal again and mining will happen

func (w *Worker) SignalStartMining() {
	select {
	case w.startMining <- true:
	default:
	}
	log.Println("mining signaled")
}

// SignalShareTX signals a share tx operation between nodes.
func (w *Worker) SignalShareTX(tx database.BlockTX) {
	select {
	case w.txSharing <- tx:
		log.Println("signalShareTX: signal sent")
	default:
	}
}

// PowOperation handles mining when a start mining signal received because of a
// submit transaction
func (w *Worker) PowOperation() {
	log.Println("worker: pow operation G started")
	defer log.Println("worker: pow operation G completed")

	for {
		select {
		case <-w.startMining:
			//check to see if worker is not shutting down
			if !w.isShuttingDown() {
				w.runPOWOperation()
			}
		case <-w.shutdown:
			log.Println("Worker: received shutdown signal")
			return
		}
	}
}

func (w *Worker) poaOperation() {
	log.Printf("worker: poaOperation: goroutine started")
	defer log.Printf("worker: poaOperation: goroutine completed")

	ticker := time.NewTicker(cycleDuration)

	resetTicker(ticker, secondsPerCycle*time.Second)

	for {
		select {
		case <-ticker.C:
			if !w.isShuttingDown() {
				w.runPOAOperation()
			}
		case <-w.shutdown:
			log.Println("worker: poaOperation: shutdown received")
			return
		}

		resetTicker(ticker, 0)
	}

}

func (w *Worker) runPOAOperation() {
	log.Println("worker:runPOAOperation: started")
	defer log.Println("worker:runPOAOperation: completed")

	//select the node to do the mining
	peer := w.selection()
	log.Printf("worker:runPOAOperation: %s selected to do the mining\n", peer)

	//if the selected is not the current node, then wait
	if peer != w.state.Host() {
		return
	}

	//if its the current node the do the mining
	length := w.state.MempoolLen()
	if length == 0 {
		log.Printf("worker:runPOAOperation: no transaction in mempool")
		return
	}

	select {
	case <-w.cancelMining:
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	//cancellation goroutine
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		select {
		case <-w.cancelMining:
			log.Printf("worker:runPOAOperation: cancelGoroutine: cancelling mining")
		case <-ctx.Done():
		}
	}()

	//mining goroutine
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		t := time.Now()
		block, err := w.state.MineNewBlock(ctx)
		duration := time.Since(t)
		log.Printf("worker:runPOAOperation: mining operation took %s\n", duration)

		if err != nil {

			switch {
			case errors.Is(err, state.ErrNoTransaction):
				log.Printf("worker: runMiningOperation: no transactions in mempool\n")
			case ctx.Err() != nil:
				log.Printf("worker: runMiningOperation: complete\n")
			default:
				log.Printf("worker: runMiningOperation: ERROR %s\n", err)
			}
			return

		}

		// The block is mined. Propose the new block to the network.
		// Log the error, but that's it.
		if err := w.state.SendBlockToPeers(block); err != nil {
			log.Printf("worker: runMiningOperation: MINING: proposeBlockToPeers: WARNING %s\n", err)
		}
	}()

	wg.Wait()
}

func (w *Worker) selection() string {
	//includes this node as well
	peers := w.state.KnownPeers()

	//hosts
	names := make([]string, len(peers))
	for i, peer := range peers {
		names[i] = peer.Host
	}

	//sort by host
	slices.Sort(names)

	//based on the latest block pick an index from register
	// Based on the latest block, pick an index number from the registry.
	h := fnv.New32a()
	h.Write([]byte(w.state.LatestBlock().Hash()))
	integerHash := h.Sum32()
	i := integerHash % uint32(len(names))

	// Return the name of the node selected.
	return names[i]
}

// isShuttingDown is used to check if a shutdown signal has been signaled.
func (w *Worker) isShuttingDown() bool {
	select {
	case <-w.shutdown:
		return true
	default:
		return false
	}
}

// runPOWOperation takes all transactions from the mempool and creates a new
// block and writes it to the database.
func (w *Worker) runPOWOperation() {
	log.Println("worker: runPOWOperation: mining a new block has started")
	defer log.Println("worker: runPOWOperation: mining a new block has completed")

	//make sure we have tx inside mempool
	length := w.state.MempoolLen()
	if length == 0 {
		log.Printf("worker: runPOWOperation: no transaction inside mempool: length=%d", length)
		return
	}

	//after this goroutine is done, check if still there are txs inside mempool, and
	//if there are, signal a new mining operation
	defer func() {
		length := w.state.MempoolLen()
		if length > 0 {
			log.Println("worker: runPOWOperation: there are txs inside mempool, signaling new mining operation")
			w.SignalStartMining()
		}
	}()

	//drain the cancel signal, since we are doing a new operation
	select {
	case <-w.cancelMining:
		log.Println("worker: runPOWOperation: drained the cancel signal")
	default:
	}

	ctx, cancel := context.WithCancel(context.Background())
	//NOTE: cancel() can be called many times its ok but at least one time must be called.
	defer cancel()

	//we need 2 goroutines, one for doing the actual POW work, another one for
	// handling cancellation

	var wg sync.WaitGroup
	wg.Add(2)

	//handles cancellation G
	go func() {
		defer func() {
			cancel() //this allows us to cancel the other G
			wg.Done()
		}()

		//blocked, until cancelMiningSignal OR ctx.Done signal then defer runs
		select {

		case <-w.cancelMining:
			log.Println("worker: runPOWOperation: cancellation G: received cancellation signal")
			//the other goroutine when its done with POW, will call cancel() as well to cancel this one.
		case <-ctx.Done():

		}
	}()

	//handles mining
	go func() {
		defer func() {
			cancel()
			wg.Done()
		}()

		t := time.Now()
		//passing down the ctx so we can cancel the mining if goroutine received
		//cancellation signal.
		block, err := w.state.MineNewBlock(ctx)
		duration := time.Since(t)

		log.Printf("worker: runPOWOperation: mining G: mining took %s\n", duration)
		if err != nil {
			switch {
			case errors.Is(err, state.ErrNoTransaction):
				log.Printf("worker: runPOWOperation: mining G: no transaction in mempool\n")
			case ctx.Err() != nil:
				log.Printf("worker: runPOWOperation: mining G: mining operation cancelled\n")
			default:
				log.Printf("worker: runPOWOperation: mining G: Error while mining: %v\n", err)
			}
			return
		}
		//mined a block successfully, share the block with the network
		if err := w.state.SendBlockToPeers(block); err != nil {
			log.Printf("runPOWOperation: sendBlockToPeers: ERROR: %v", err)
		}
	}()
	wg.Wait()
}

// Sync updates the peers list, mempool and blocks.
func (w *Worker) Sync() {
	log.Println("worker: sync: started")
	defer log.Println("worker: sync: completed")

	for _, peer := range w.state.KnownExternalPeers() {
		//get the status of the peer
		peerStatus, err := w.state.RequestPeerStatus(peer)
		if err != nil {
			log.Printf("worker: sync: requestPeerStatus: [%s]:%v\n", peer.Host, err)
		}

		//add new peers
		w.addNewPeers(peerStatus.KnownPeers)

		//ask for mempool of that peer
		pool, err := w.state.RequestMempool(peer)
		if err != nil {
			log.Printf("worker: sync: requestMempool: [%s]:%v\n", peer.Host, err)
		}

		//update current node's mempool with those transactions
		for _, tx := range pool {
			log.Printf("worker: sync: mempool: %s: TX[%s]\n", peer.Host, tx.SignatureString()[:16])
			w.state.UpsertMempool(tx)
		}

		//if peer has some blocks that we do not have, we ask for them
		if peerStatus.LatestBlockNumber > w.state.LatestBlock().Header.Number {
			log.Printf("worker: sync: retrieve peer's blocks: %s: latestBlockNumber: %d\n", peer.Host, peerStatus.LatestBlockNumber)
			if err := w.state.RequestPeerBlocks(peer); err != nil {
				log.Printf("worker: sync: retrieve peer's blocks: %s: ERROR: %v\n", peer.Host, err)
			}
		}
	}

	//tell the rest of network that this node is ready to participate
	w.state.SendNodeAvailableToPeers()
}

func (w *Worker) addNewPeers(knownPeers []peer.Peer) error {
	for _, peer := range knownPeers {
		//skip the current node
		if !peer.Match(w.state.Host()) {
			continue
		}
		if w.state.AddKnownPeer(peer) {
			log.Printf("worker: addNewPeers: add node[%s]\n", peer.Host)
		}
	}

	return nil
}

func (w *Worker) ShareTxOperation() {
	log.Println("shareTxOperation: goroutine started")
	defer log.Println("shareTxOperation: goroutine finished")

	for {
		select {
		case tx := <-w.txSharing:
			if !w.isShuttingDown() {
				w.state.SendTxToPeers(tx)
			}
		case <-w.shutdown:
			log.Println("shareTxOperation: received shutdown signal")
			return
		}
	}
}

func (w *Worker) peerSyncOperation() {
	log.Println("worker: peerSyncOperation: goroutine started")
	defer log.Println("worker: peerSyncOperation: goroutine completed")

	for {
		select {
		case <-w.ticker.C:
			if !w.isShuttingDown() {
				w.peerSyncing()
			}
		case <-w.shutdown:
			log.Println("worker: peerSyncOperation: shutdown received")
			return
		}
	}
}

// peerSyncing updates the peer list.
func (w *Worker) peerSyncing() {
	log.Println("worker: peerSyncing: started")
	defer log.Println("worker: peerSyncing: completed")

	for _, peer := range w.state.KnownExternalPeers() {
		peerStatus, err := w.state.RequestPeerStatus(peer)
		if err != nil {
			log.Printf("worker: peerSyncing: requestPeerStatus: %s: ERROR: %s", peer.Host, err)

			//remove it from list
			w.state.RemoveKnownPeer(peer)
			continue
		}

		//add the updated list of peers
		w.addNewPeers(peerStatus.KnownPeers)
	}

	//share tha the current node is also available
	w.state.SendNodeAvailableToPeers()
}

// ==============================================================================
func resetTicker(ticker *time.Ticker, waitOnSecond time.Duration) {
	nextTick := time.Now().Add(cycleDuration).Round(waitOnSecond)
	diff := time.Until(nextTick)
	ticker.Reset(diff)
}
