package database

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"crypto/ecdsa"

	"maps"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hamidoujand/chaingo/genesis"
	"github.com/hamidoujand/chaingo/merkle"
	"github.com/hamidoujand/chaingo/signature"
)

//==============================================================================
// AccountID

// AccountID represents the last 20 bytes of the public key.
type AccountID string

func NewAccountID(hex string) (AccountID, error) {
	a := AccountID(hex)
	if !a.IsValid() {
		return "", fmt.Errorf("invalid accountID format")
	}
	return a, nil
}

func PublicToAccountID(pub ecdsa.PublicKey) AccountID {
	return AccountID(crypto.PubkeyToAddress(pub).String())
}

func (id AccountID) IsValid() bool {
	const addressLength = 20
	if has0xPrefix(id) {
		id = id[2:] //remove the prefix then.
	}

	//1 byte = 2 hex char
	return len(id) == 2*addressLength && isHex(id)
}

func has0xPrefix(id AccountID) bool {
	return len(id) >= 2 && id[0] == '0' && (id[1] == 'x' || id[1] == 'X')
}

func isHex(id AccountID) bool {
	if len(id)%2 != 0 {
		return false
	}

	for _, c := range []byte(id) {
		if !isHexCharacter(c) {
			return false
		}
	}

	return true
}

func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

//==============================================================================

// TX represents an unsigned transaction.
type TX struct {
	ChainID uint16    `json:"chain_id"` //Prevents replay attacks across different blockchains.
	Nonce   uint64    `json:"nonce"`    //Acts as a unique counter for transactions from the same account and ensures transaction ordering.
	FromID  AccountID `json:"from"`     //Nodes verify that the signature matches this address (to prevent impersonation).
	ToID    AccountID `json:"to"`       //The recipient’s account address.
	Value   uint64    `json:"value"`    //The amount of cryptocurrency being sent.
	Tip     uint64    `json:"tip"`      //A priority fee (tip) to incentivize miners to include this transaction faster.
	Data    []byte    `json:"data"`     //Stores arbitrary data.
}

func NewTX(chainID uint16, nonce uint64, from AccountID, to AccountID, value uint64, tip uint64, data []byte) (TX, error) {
	tx := TX{
		ChainID: chainID,
		Nonce:   nonce,
		FromID:  from,
		ToID:    to,
		Value:   value,
		Tip:     tip,
		Data:    data,
	}

	return tx, nil
}

func (tx TX) Sign(privateKey *ecdsa.PrivateKey) (SignedTX, error) {
	v, r, s, err := signature.Sign(tx, privateKey)
	if err != nil {
		return SignedTX{}, fmt.Errorf("signing tx: %w", err)
	}

	stx := SignedTX{
		TX: tx,
		V:  v,
		R:  r,
		S:  s,
	}

	return stx, nil
}

//==============================================================================
// Signed Transaction

// SignedTX represents a digitally signed transaction in the blockchain.
type SignedTX struct {
	TX
	V *big.Int `json:"v"`  //Recovery identifier, either 29 or 30 with chaingo.
	R *big.Int `json:"r"`  //First part of the ECDSA signature (random point on the elliptic curve).
	S *big.Int `json:"s" ` //Second part of the ECDSA signature (proof of signing authority).
}

// Validate verifies that received transaction over the wire has proper signature.
func (stx SignedTX) Validate(chainID uint16) error {
	//check if the TX meant for this blockchain.
	if stx.ChainID != chainID {
		return fmt.Errorf("invalid chainID: %d", stx.ChainID)
	}

	//accountIDs
	if !stx.FromID.IsValid() {
		return fmt.Errorf("fromID is not in proper format")
	}

	if !stx.ToID.IsValid() {
		return fmt.Errorf("toID is not in proper format")
	}

	if stx.FromID == stx.ToID {
		return fmt.Errorf("invalid transaction, sending money to yourself, from %s to %s", stx.FromID, stx.ToID)
	}

	//validate the signature from structure point of view
	if err := signature.VerifySignature(stx.V, stx.R, stx.S); err != nil {
		return fmt.Errorf("verifySignature: %w", err)
	}

	addr, err := signature.ExtractAddress(stx.TX, stx.V, stx.R, stx.S)
	if err != nil {
		return fmt.Errorf("extractAddress: %w", err)
	}

	if addr != string(stx.FromID) {
		return errors.New("signature address does not match the FromID address")
	}

	return nil
}

func (stx SignedTX) SignatureString() string {
	return signature.SignatureString(stx.V, stx.R, stx.S)
}

func (stx SignedTX) String() string {
	return fmt.Sprintf("%s:%d", stx.FromID, stx.Nonce)
}

//==============================================================================
// Account

// Account represents all information stored in database for an individual account.
type Account struct {
	AccountID AccountID
	Nonce     uint64 //represents the TX number.
	Balance   uint64
}

func newAccount(accountID AccountID, Balance uint64) Account {
	return Account{
		AccountID: accountID,
		Balance:   Balance,
	}
}

//==============================================================================
// Database

// Database manages all the accounts who transacted on the blockchain.
// since we store all blocks on disk, we can read them and rebuild our database every time.
type Database struct {
	mu       sync.RWMutex
	genesis  genesis.Genesis
	accounts map[AccountID]Account
}

// New creates a new database and applies the genesis.
func New(genesis genesis.Genesis) (*Database, error) {
	db := Database{
		genesis:  genesis,
		accounts: make(map[AccountID]Account),
	}

	for accountSTR, balance := range genesis.Balances {
		accountID, err := NewAccountID(accountSTR)
		if err != nil {
			return nil, fmt.Errorf("invalid accountID: %w", err)
		}

		db.accounts[accountID] = newAccount(accountID, balance)
	}

	return &db, nil
}

func (db *Database) Remove(accountID AccountID) {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.accounts, accountID)
}

func (db *Database) Query(accountID AccountID) (Account, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	account, ok := db.accounts[accountID]
	if !ok {
		return Account{}, fmt.Errorf("account with ID %s not found", accountID)
	}

	return account, nil
}

func (db *Database) Copy() map[AccountID]Account {
	db.mu.RLock()
	defer db.mu.RUnlock()
	cp := make(map[AccountID]Account, len(db.accounts))

	maps.Copy(cp, db.accounts)

	return cp
}

//==============================================================================
// Block (batch of transactions)

// BlockTX represents a TX inside of the block.
type BlockTX struct {
	SignedTX
	Timestamp uint64 `json:"timestamp"` //time that the transaction was received.
	GasPrice  uint64 `json:"gas_price"` //price for single unite of gas.
	GasUnits  uint64 `json:"gas_units"` //number of units of gas used for this transaction.
}

// NewBlockTX constructs a new block transaction.
func NewBlockTX(signedTx SignedTX, gasPrice uint64, uintOfGas uint64) BlockTX {
	return BlockTX{
		SignedTX:  signedTx,
		Timestamp: uint64(time.Now().UTC().UnixMilli()),
		GasPrice:  gasPrice,
		GasUnits:  uintOfGas,
	}
}

// Hash implements the merkle Hashable interface for providing a hash
// of a block transaction.
func (btx BlockTX) Hash() ([]byte, error) {
	str := signature.Hash(btx)

	//remove the 0x from the hash string
	return hexutil.Decode(str[2:])
}

// Equals implements the merkle Hashable interface for providing an equality
// check between two block transactions. If the nonce and signatures are the
// same, the two blocks are the same.
func (btx BlockTX) Equals(other BlockTX) bool {
	txSig := signature.ToSignatureBytes(btx.V, btx.R, btx.S)
	otherSig := signature.ToSignatureBytes(other.V, other.R, other.S)

	return btx.Nonce == other.Nonce && bytes.Equal(txSig, otherSig)
}

// BlockHeader represents common information required by each block.
type BlockHeader struct {
	//Represents the block height (e.g., 0 for genesis, 1 for the next block).
	Number uint64 `json:"number"`
	// The SHA-256 hash (or similar) of the previous block’s header.
	// Forms the "chain" in blockchain by linking blocks immutably.
	PrevBlockHash string `json:"prev_block_hash"`
	Timestamp     uint64 `json:"timestamp"`
	// The miner’s address who receives the MiningReward
	BeneficiaryID AccountID `json:"beneficiary"`
	// Controls how hard the Proof-of-Work (PoW) puzzle is.
	// Higher value = more leading zeros required in the block hash.
	Difficulty   uint16 `json:"difficulty"`
	MiningReward uint64 `json:"mining_reward"`
	// Merkle root hash of the entire accounts and balances
	// Allows lightweight verification of state without storing all data.
	StateRoot string `json:"state_root"`
	// Merkle root hash of all transactions in the block.
	// Ensures transactions are tamper-proof.
	TransRoot string `json:"trans_root"`
	// A random value miners change to find a valid block hash.
	// In PoW, this is the "guess" to solve the cryptographic puzzle.
	Nonce uint64 `json:"nonce"`
}

// BlockData represents a block that can be serialized on disk and over the network.
type BlockData struct {
	Hash   string      `json:"hash"`
	Header BlockHeader `json:"block"`
	Trans  []BlockTX   `json:"trans"`
}

// Block represents a block inside memory and transactions will be inside of a merkle tree.
type Block struct {
	Header     BlockHeader
	MerkleTree *merkle.Tree[BlockTX]
}

// Hash returns the unique hash for the Block.
func (b Block) Hash() string {
	//first block always has a Zero hash
	if b.Header.Number == 0 {
		return signature.ZeroHash
	}

	//we only hash the header not the entire block and transactions, so when
	//we need to check, we only need the header not the entire block.
	return signature.Hash(b.Header)
}

// NewBlockData construct a BlockData from a Block.
func NewBlockData(block Block) BlockData {
	blockData := BlockData{
		Hash:   block.Hash(),
		Header: block.Header,
		Trans:  block.MerkleTree.Values(),
	}

	return blockData
}

// ToBlock takes a BlockData from disk or network and converts it to Block to use inside memory.
func ToBlock(blockData BlockData) (Block, error) {
	tree, err := merkle.NewTree(blockData.Trans)
	if err != nil {
		return Block{}, fmt.Errorf("newTree: %w", err)
	}

	block := Block{
		Header:     blockData.Header,
		MerkleTree: tree,
	}

	return block, nil
}
