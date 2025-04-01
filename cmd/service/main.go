package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hamidoujand/chaingo/database"
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

	v, r, s, err := toVRSFromHEX(hexed)
	if err != nil {
		return fmt.Errorf("toVRS: %w", err)
	}

	fmt.Printf("v:%d\n", v)
	fmt.Printf("r:%d\n", r)
	fmt.Printf("s:%d\n", s)

	fmt.Println("==============================TX===============================")

	hamidTX, err := database.NewTX(
		1,
		1,
		"0xB7098929d914880eF9A18026F2290A9F23390D42",
		"0xB7098929d914880eF9A18026F2290A9F23390D42",
		1000,
		0,
		nil,
	)

	if err != nil {
		return fmt.Errorf("newTX: %w", err)
	}

	signedTX, err := hamidTX.Sign(private)
	if err != nil {
		return fmt.Errorf("signTX: %w", err)
	}

	fmt.Printf("%+v\n", signedTX)
	return nil
}

func toVRSFromHEX(signature string) (v, r, s *big.Int, err error) {
	sig, err := hex.DecodeString(signature[2:]) // skip 0x
	if err != nil {
		return nil, nil, nil, err
	}

	r = big.NewInt(0).SetBytes(sig[:32])
	s = big.NewInt(0).SetBytes(sig[32:64])
	v = big.NewInt(0).SetBytes([]byte{sig[64]})

	return v, r, s, nil
}
