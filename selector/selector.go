package selector

import (
	"fmt"
	"slices"

	"github.com/hamidoujand/chaingo/database"
)

const (
	StrategyTip = "tip"
)

// Selector takes a mempool of transaction grouped by accountID and based on selected strategy orders those TX.
type Selector func(transactions map[database.AccountID][]database.BlockTX, count int) []database.BlockTX

var strategies = map[string]Selector{
	StrategyTip: tipSelector,
}

func Retrieve(strategy string) (Selector, error) {
	fn, exists := strategies[strategy]
	if !exists {
		return nil, fmt.Errorf("strategy %q not found", strategy)
	}

	return fn, nil
}

//==============================================================================
// tip selector

func tipSelector(transactions map[database.AccountID][]database.BlockTX, count int) []database.BlockTX {
	//step 1: sort each accounts transaction by nonce
	for _, txs := range transactions {
		if len(txs) > 1 {
			sortByNonce(txs)
		}
	}

	//step 2: group into rows (transactions with the same nonce priority)
	// grouping transactions into "rows" where each row
	// contains one transaction per account (regardless of nonce values).
	var rows [][]database.BlockTX
	for {
		var row []database.BlockTX
		for acc := range transactions {
			if len(transactions[acc]) > 0 {
				//take the lowest nonce of each account store them inside row.
				row = append(row, transactions[acc][0])
				//remove that lowest nonce from each account
				transactions[acc] = transactions[acc][1:]
			}
		}
		//if there is no more tx inside of ROW, that means we collected TXs of all accounts
		if len(row) == 0 {
			break
		}
		rows = append(rows, row)
	}

	selected := make([]database.BlockTX, 0, count)
	for _, row := range rows {
		if len(selected) >= count {
			break
		}

		//sort each row by tip (row= first tx of each account)
		sortByTip(row)
		remaining := count - len(selected)

		if len(row) > remaining {
			selected = append(selected, row[:remaining]...)
			break
		}

		selected = append(selected, row...)
	}

	return selected
}

// sorts transactions by nonce (ascending)
func sortByNonce(txs []database.BlockTX) {
	slices.SortFunc(txs, func(a, b database.BlockTX) int {
		return int(a.Nonce - b.Nonce)
	})
}

// sorts transactions by tip (descending)
func sortByTip(txs []database.BlockTX) {
	slices.SortFunc(txs, func(a, b database.BlockTX) int {
		return int(b.Tip - a.Tip) //higher tips first
	})
}
