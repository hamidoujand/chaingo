package genesis

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Genesis defines the starting configuration of the blockchain.
type Genesis struct {
	Date          time.Time         `json:"date"`            // when the blockchain was created.
	ChainID       uint16            `json:"chain_id"`        // a unique ID for then blockchain (like a "network ID").
	TransPerBlock uint16            `json:"trans_per_block"` // max number of transaction inside of single block.
	Difficulty    uint16            `json:"difficulty"`      // How hard it is to mine a block (higher = harder).
	MiningReward  uint64            `json:"mining_reward"`   // Reward for mining a block.
	GasPrice      uint64            `json:"gas_price"`       // Fee paid for each transaction mined into a block.
	Balances      map[string]uint64 `json:"balances"`
}

// Load reads the genesis file.
func Load() (Genesis, error) {
	bs, err := os.ReadFile("genesis.json")
	if err != nil {
		return Genesis{}, fmt.Errorf("readFile: %w", err)
	}

	var gen Genesis
	if err := json.Unmarshal(bs, &gen); err != nil {
		return Genesis{}, fmt.Errorf("unmarshal: %w", err)
	}

	return gen, nil
}
