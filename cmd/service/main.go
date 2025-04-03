package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/genesis"
	"github.com/hamidoujand/chaingo/state"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {

	//==========================================================================
	// Blockchain
	beneficiary := os.Getenv("CHAINGO_BENEFICIARY")
	if beneficiary == "" {
		return errors.New("missing env CHAINGO_BENEFICIARY")
	}

	//load the beneficiary's private key .
	path := fmt.Sprintf("block/%s.ecdsa", beneficiary)
	privateKey, err := crypto.LoadECDSA(path)
	if err != nil {
		return fmt.Errorf("loadECDSA: %w", err)
	}

	genesis, err := genesis.Load()
	if err != nil {
		return fmt.Errorf("loading genesis: %w", err)
	}

	state, err := state.New(state.Config{
		BeneficiaryID: database.PublicToAccountID(privateKey.PublicKey),
		Genesis:       genesis,
	})
	fmt.Printf("%+v\n", state)
	return nil
}
