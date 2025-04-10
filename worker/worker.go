// Package worker provides concurrent support for mining, peer updates, tx sharing.
package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/hamidoujand/chaingo/state"
)

// Worker is the manager of all goroutines created by this package.
type Worker struct {
	state        *state.State
	wg           sync.WaitGroup
	shutdown     chan struct{}
	startMining  chan bool
	cancelMining chan bool
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

// Run creates a worker and attach the worker to the state.
func Run(state *state.State) {
	w := Worker{
		state:        state,
		shutdown:     make(chan struct{}),
		startMining:  make(chan bool, 1),
		cancelMining: make(chan bool, 1),
	}

	//register worker to the state
	state.Worker = &w

	//set of operation to do
	operations := []func(){
		w.PowOperation,
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
		_, err := w.state.MineNewBlock(ctx)
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
		//mined a block successfully

	}()
	wg.Wait()
}
