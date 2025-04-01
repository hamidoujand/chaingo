package signature

import (
	ecd "crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
)

// our unique chain ID.
const chaingoID = 29

func Sign(tx any, privateKey *ecd.PrivateKey) (v, r, s *big.Int, err error) {
	//marshal it
	bs, err := json.Marshal(tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshalling tx: %w", err)
	}

	//to indicate that this transaction is meant for Chaingo blockchain.
	stamp := fmt.Appendf(nil, "\x19Chaingo Signed Message:\n%d", len(bs))

	//digest it to 32 bytes
	digest := crypto.Keccak256(stamp, bs)

	//sign
	signature, err := crypto.Sign(digest, privateKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("signing ECDSA: %w", err)
	}

	v, r, s = toSignatureRSV(signature)

	return v, r, s, nil
}

func toSignatureRSV(sig []byte) (v, r, s *big.Int) {
	r = big.NewInt(0).SetBytes(sig[:32])
	s = big.NewInt(0).SetBytes(sig[32:64])

	//EIP-155-style V
	recoverID := int(sig[64]) // 0 OR 1
	result := int64(chaingoID*2 + recoverID + 35)
	v = big.NewInt(result)

	return v, r, s
}
