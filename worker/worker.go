// Package worker provides concurrent support for mining, peer updates, tx sharing.
package worker

import (
	"log"
	"sync"

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

	log.Println("terminating worker goroutines")
	close(w.shutdown)
	w.wg.Wait()
}

// SignalCancelMining implements state.Worker.
func (w *Worker) SignalCancelMining() {
	panic("unimplemented")
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
		case <-w.shutdown:
			return
		}
	}
}
