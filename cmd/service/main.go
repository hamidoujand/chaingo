package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type Tx struct {
	FromID string `json:"from"`
	ToID   string `json:"to"`
	Value  uint64 `json:"value"`
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	tx := Tx{
		FromID: "Hamid",
		ToID:   "Wolf",
		Value:  200,
	}

	//load a private key to sign the transaction with it.
	private, err := crypto.LoadECDSA("zblock/hamid.ecdsa")
	if err != nil {
		return fmt.Errorf("loadECDSA: %w", err)
	}

	//marshal the transaction data
	bs, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("marshalling tx: %w", err)
	}

	//to indicate that this transaction is meant for Chaingo blockchain.
	stamp := fmt.Appendf(nil, "\x19Chaingo Signed Message:\n%d", len(bs))

	//digest it to 32 bytes
	digest := crypto.Keccak256(stamp, bs)

	//sign
	signature, err := crypto.Sign(digest, private)
	if err != nil {
		return fmt.Errorf("sign ECDSA: %w", err)
	}

	hexed := hexutil.Encode(signature)

	fmt.Printf("\n===================Signature=================\n%s\n", hexed)

	//other side needs to get the Public Key (Address) out of this signature.
	publicKey, err := crypto.SigToPub(digest, signature)
	if err != nil {
		return fmt.Errorf("sigToPub: %w", err)
	}

	//get the address
	addr := crypto.PubkeyToAddress(*publicKey).String()
	fmt.Printf("\n======================Address===============\n%s\n", addr)

	return nil
}
