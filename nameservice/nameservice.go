package nameservice

import (
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hamidoujand/chaingo/database"
)

type Nameservice struct {
	accounts map[database.AccountID]string
}

// New takes the root which is the dir containing private keys and returns a nameservice.
func New(root string) (*Nameservice, error) {
	ns := Nameservice{
		accounts: make(map[database.AccountID]string),
	}

	fn := func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".ecdsa" {
			return nil
		}

		private, err := crypto.LoadECDSA(path)
		if err != nil {
			return fmt.Errorf("loadECDSA: %w", err)
		}

		accountID := database.PublicToAccountID(private.PublicKey)
		ns.accounts[accountID] = strings.TrimSuffix(filepath.Base(path), ".ecdsa")
		return nil
	}

	if err := filepath.WalkDir(root, fn); err != nil {
		return nil, fmt.Errorf("walkDir: %w", err)
	}

	return &ns, nil
}

func (ns *Nameservice) Lookup(accountID database.AccountID) string {
	name, exits := ns.accounts[accountID]
	if !exits {
		return string(accountID)
	}
	return name
}

func (ns *Nameservice) Copy() map[database.AccountID]string {
	dest := make(map[database.AccountID]string, len(ns.accounts))
	maps.Copy(dest, ns.accounts)
	return dest
}
