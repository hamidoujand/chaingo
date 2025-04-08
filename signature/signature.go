package signature

import (
	ecd "crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// our unique chain ID.
const chaingoID = 29
const ZeroHash = "0x0000000000000000000000000000000000000000000000000000000000000000"

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

func VerifySignature(v, r, s *big.Int) error {
	recoverID := v.Uint64() - chaingoID*2 - 35

	//check the V value to be either 0 or 1.
	if recoverID != 0 && recoverID != 1 {
		return errors.New("invalid recovery id")
	}

	// validates the structure of the signature.
	if !crypto.ValidateSignatureValues(byte(recoverID), r, s, false) {
		return errors.New("invalid signature values")
	}
	return nil
}

func ExtractAddress(tx any, v, r, s *big.Int) (string, error) {
	//marshal it
	bs, err := json.Marshal(tx)
	if err != nil {
		return "", fmt.Errorf("marshalling tx: %w", err)
	}

	//stamp
	stamp := fmt.Appendf(nil, "\x19Chaingo Signed Message:\n%d", len(bs))

	//digest hash
	digest := crypto.Keccak256(stamp, bs)

	//convert RSV to 65 bytes format
	sig := ToSignatureBytes(v, r, s)

	//extract the public key
	publicKey, err := crypto.SigToPub(digest, sig)
	if err != nil {
		return "", fmt.Errorf("sigToPub: %w", err)
	}

	return crypto.PubkeyToAddress(*publicKey).String(), nil
}

// SignatureString returns the signature as string.
func SignatureString(v, r, s *big.Int) string {
	//without chaingoID
	sig := ToSignatureBytes(v, r, s)

	//with chaingoID
	sig[64] = byte(v.Uint64())

	return hexutil.Encode(sig)
}

// Hash returns a unique string to every value.
func Hash(val any) string {
	bs, err := json.Marshal(val)
	if err != nil {
		return ZeroHash
	}

	hash := sha256.Sum256(bs)
	return hexutil.Encode(hash[:])
}

// ToSignatureBytes converts the r, s, v values into a slice of bytes
// with the removal of the ChaingoID.
func ToSignatureBytes(v, r, s *big.Int) []byte {
	sig := make([]byte, crypto.SignatureLength)

	//R
	r.FillBytes(sig[:32])

	//S
	s.FillBytes(sig[32:64])

	recoverID := v.Uint64() - (chaingoID*2 + 35)
	sig[64] = byte(recoverID)

	return sig
}

// ==============================================================================

func toSignatureRSV(sig []byte) (v, r, s *big.Int) {
	r = big.NewInt(0).SetBytes(sig[:32])
	s = big.NewInt(0).SetBytes(sig[32:64])

	//EIP-155-style V
	recoverID := int(sig[64]) // 0 OR 1
	result := int64(chaingoID*2 + recoverID + 35)
	v = big.NewInt(result)

	return v, r, s
}
