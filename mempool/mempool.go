package mempool

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/hamidoujand/chaingo/database"
	"github.com/hamidoujand/chaingo/selector"
)

// Mempool represents a cache of transactions organized by account:none.
// different blockchains have different algorithms to limit the size of mempool
// some based on memory that mempool consumes and some based on the number of
// transactions. it that limit is met, then either old transactions or least
// return investment will drop to make room for new transactions.
type Mempool struct {
	mu       sync.RWMutex
	pool     map[string]database.BlockTX
	selector selector.Selector
}

func New(strategy string) (*Mempool, error) {
	selectorFn, err := selector.Retrieve(strategy)
	if err != nil {
		return nil, fmt.Errorf("retrieve selector: %w", err)
	}

	m := Mempool{
		pool:     make(map[string]database.BlockTX),
		selector: selectorFn,
	}
	return &m, nil
}

// Count returns the current number of transactions inside mempool.
func (m *Mempool) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pool)
}

// Upsert adds or updates a transaction from mempool.
func (m *Mempool) Upsert(tx database.BlockTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	//unique key : accountID:nonce
	key := fmt.Sprintf("%s:%d", tx.FromID, tx.Nonce)

	//Ethereum requires 10% bump in the tip to replace an existing transaction
	//this also limits users for this sort of behavior.
	if etx, exists := m.pool[key]; exists {
		//10% to the tip
		if tx.Tip < etx.Tip*110/100 {
			return errors.New("replacing TX requires a 10%% bump in the tip")
		}
		//10% to the gas price
		if tx.GasPrice < etx.GasPrice*100/100 {
			return errors.New("replacing TX requires a 10%% bump in the gas price")
		}
	}

	m.pool[key] = tx
	return nil
}

// Delete removes a tx from the mempool .
func (m *Mempool) Delete(tx database.BlockTX) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%d", tx.FromID, tx.Nonce)
	delete(m.pool, key)
}

// Truncate removes all transactions from the mempool.
func (m *Mempool) Truncate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pool = make(map[string]database.BlockTX)
}

func (m *Mempool) PickBest(count int) []database.BlockTX {
	group := make(map[database.AccountID][]database.BlockTX)

	m.mu.RLock()

	if count == 0 {
		count = len(m.pool)
	}

	for key, tx := range m.pool {
		account := strings.Split(key, ":")[0]
		accountID := database.AccountID(account)
		group[accountID] = append(group[accountID], tx)
	}

	m.mu.RUnlock()
	return m.selector(group, count)
}
