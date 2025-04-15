package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/genesis"
	"github.com/hamidoujand/chaingo/handlers"
	"github.com/hamidoujand/chaingo/nameservice"
	"github.com/hamidoujand/chaingo/peer"
	"github.com/hamidoujand/chaingo/selector"
	"github.com/hamidoujand/chaingo/state"
	"github.com/hamidoujand/chaingo/storage/disk"
	"github.com/hamidoujand/chaingo/worker"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	//==========================================================================
	// Environment

	beneficiary := os.Getenv("CHAINGO_BENEFICIARY")
	if beneficiary == "" {
		return errors.New("missing env CHAINGO_BENEFICIARY")
	}

	strategy := os.Getenv("CHAINGO_SELECTOR_STRATEGY")
	if strategy == "" {
		strategy = selector.StrategyTip
	}

	publicHost := os.Getenv("CHAINGO_PUBLIC_HOST")
	if publicHost == "" {
		publicHost = "0.0.0.0:8000"
	}

	privateHost := os.Getenv("CHAINGO_PRIVATE_HOST")
	if privateHost == "" {
		privateHost = "0.0.0.0:9000"
	}

	keysFolder := os.Getenv("CHAINGO_KEYS_DIR")
	if keysFolder == "" {
		keysFolder = "block"
	}

	storagePath := os.Getenv("CHAINGO_STORAGE_DIR")
	if storagePath == "" {
		storagePath = "block/miner1"
	}

	consensus := os.Getenv("CHAINGO_CONSENSUS")
	if consensus == "" {
		consensus = state.ConsensusPOW
	}

	originPeers := os.Getenv("CHAINGO_ORIGIN_PEER")
	if originPeers == "" {
		originPeers = "0.0.0.0:9000"
	}

	originPeersList := strings.Split(originPeers, ",")

	//==========================================================================
	// Storage
	disk, err := disk.New(storagePath)
	if err != nil {
		return fmt.Errorf("new disk: %w", err)
	}

	//==========================================================================
	// Blockchain

	//load the beneficiary's private key .
	path := fmt.Sprintf("block/%s.ecdsa", beneficiary)
	privateKey, err := crypto.LoadECDSA(path)
	if err != nil {
		return fmt.Errorf("loadECDSA: %w", err)
	}

	peerSet := peer.NewPeerSet()
	for _, host := range originPeersList {
		peerSet.Add(peer.New(host))
	}

	peerSet.Add(peer.New(privateHost))

	genesis, err := genesis.Load()
	if err != nil {
		return fmt.Errorf("loading genesis: %w", err)
	}

	state, err := state.New(state.Config{
		BeneficiaryID: database.PublicToAccountID(privateKey.PublicKey),
		Genesis:       genesis,
		Strategy:      strategy,
		Storage:       disk,
		KnownPeers:    peerSet,
		Host:          privateHost,
		Consensus:     consensus,
	})

	if err != nil {
		return fmt.Errorf("new state: %w", err)
	}

	defer state.Shutdown()

	worker.Run(state)

	//==========================================================================
	// Nameservice
	ns, err := nameservice.New(keysFolder)
	if err != nil {
		return fmt.Errorf("new nameservice: %w", err)
	}

	for acc, name := range ns.Copy() {
		log.Printf("name=%s account=%s", name, acc)
	}

	//==========================================================================
	// Mux

	publicMux := handlers.PublicMux(handlers.MuxConfig{
		State: state,
		NS:    ns,
	})

	//==========================================================================
	// Server

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	serverErrs := make(chan error, 1)
	//===================
	// Public Server
	publicServer := &http.Server{
		Addr:        publicHost,
		Handler:     http.TimeoutHandler(publicMux, time.Second*30, "timed out"),
		ReadTimeout: time.Second * 10, //TODO: needs load testing.
		IdleTimeout: time.Second * 60, //TODO: needs load testing.
	}

	go func() {
		log.Printf("public server starting at: %s", publicHost)
		if err := publicServer.ListenAndServe(); err != nil {
			serverErrs <- fmt.Errorf("listenAndServe public: %w", err)
		}
	}()

	//===================
	// Private Server

	privateMux := handlers.PrivateMux(handlers.MuxConfig{
		State: state,
		NS:    ns,
	})

	privateServer := &http.Server{
		Addr:        privateHost,
		Handler:     http.TimeoutHandler(privateMux, time.Second*30, "timed out"),
		ReadTimeout: time.Second * 10, //TODO: needs load testing.
		IdleTimeout: time.Second * 60, //TODO: needs load testing.
	}

	go func() {
		log.Printf("private server starting at: %s", privateHost)
		if err := privateServer.ListenAndServe(); err != nil {
			serverErrs <- fmt.Errorf("listenAndServer private: %w", err)
		}
	}()

	select {
	case err := <-serverErrs:
		return err
	case sig := <-shutdown:
		log.Printf("received signal %q, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel()

		if err := privateServer.Shutdown(ctx); err != nil {
			_ = privateServer.Close()
			return fmt.Errorf("private server shutdown: %w", err)
		}

		if err := publicServer.Shutdown(ctx); err != nil {
			_ = publicServer.Close()
			return fmt.Errorf("public server shutdown: %w", err)
		}
	}

	return nil
}
